package tools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ExecTool runs shell commands with a timeout.
// For safety:
// - prefer array form: {"cmd": ["ls", "-la"]}
// - string form (shell) is disallowed to avoid shell injection
// - blacklist dangerous program names (rm, sudo, dd, mkfs, shutdown, reboot)
// - arguments containing absolute paths, ~ or .. are rejected
// - optional allowedDir enforces a working directory

type ExecTool struct {
	timeout    time.Duration
	allowedDir string
}

func NewExecTool(timeoutSecs int) *ExecTool {
	return &ExecTool{timeout: time.Duration(timeoutSecs) * time.Second}
}

// NewExecToolWithWorkspace creates an ExecTool restricted to the provided workspace directory.
func NewExecToolWithWorkspace(timeoutSecs int, allowedDir string) *ExecTool {
	return &ExecTool{timeout: time.Duration(timeoutSecs) * time.Second, allowedDir: allowedDir}
}

func (t *ExecTool) Name() string { return "exec" }
func (t *ExecTool) Description() string {
	return "Execute shell commands (array form only, restricted for safety)"
}

func (t *ExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"cmd": map[string]interface{}{
				"type":        "array",
				"description": "Command as array [program, arg1, arg2, ...]. String form is disallowed for security.",
				"items": map[string]interface{}{
					"type": "string",
				},
				"minItems": 1,
			},
		},
		"required": []string{"cmd"},
	}
}

var dangerous = map[string]struct{}{
	"rm":       {},
	"sudo":     {},
	"dd":       {},
	"mkfs":     {},
	"shutdown": {},
	"reboot":   {},
	"bash":     {},
	"sh":       {},
	"zsh":      {},
	"nc":       {},
	"netcat":   {},
	"nmap":     {},
}

func isDangerousProg(prog string) bool {
	base := filepath.Base(prog)
	base = strings.ToLower(base)
	_, ok := dangerous[base]
	return ok
}

// isInterpreter returns true for programs that accept -c with inline source code.
var interpreters = map[string]struct{}{
	"python":  {},
	"python3": {},
	"perl":    {},
	"ruby":    {},
	"node":    {},
}

func isInterpreter(prog string) bool {
	base := filepath.Base(prog)
	base = strings.ToLower(base)
	_, ok := interpreters[base]
	return ok
}

// isPackageManager returns true for package managers whose arguments
// (package names, flags like --user, --break-system-packages) are safe.
var packageManagers = map[string]struct{}{
	"pip":  {},
	"pip3": {},
	"uv":   {},
}

func isPackageManager(prog string) bool {
	base := filepath.Base(prog)
	base = strings.ToLower(base)
	_, ok := packageManagers[base]
	return ok
}

func hasUnsafeArg(s string) bool {
	// A more aggressive check: reject any arg containing path separators,
	// home expansion or parent directory references anywhere.
	// We also reject shell characters that could be used for chaining.
	if strings.Contains(s, "/") || strings.Contains(s, "..") || strings.Contains(s, "~") {
		return true
	}
	// Reject common shell meta-characters that might bypass the array-only restriction
	// if the binary itself invokes a shell (e.g. some scripts).
	meta := []string{";", "&", "|", ">", "<", "$", "`"}
	for _, m := range meta {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	cmdRaw, ok := args["cmd"]
	if !ok {
		return "", fmt.Errorf("exec: 'cmd' argument required")
	}

	// Disallow shell-string commands for safety
	if _, ok := cmdRaw.(string); ok {
		return "", errors.New("exec: string commands are disallowed; use array form")
	}

	var argv []string
	switch v := cmdRaw.(type) {
	case []interface{}:
		if len(v) == 0 {
			return "", fmt.Errorf("exec: empty cmd array")
		}
		for _, a := range v {
			s, ok := a.(string)
			if !ok {
				return "", fmt.Errorf("exec: cmd array must contain strings only")
			}
			argv = append(argv, s)
		}
	default:
		return "", fmt.Errorf("exec: unsupported cmd type")
	}

	prog := argv[0]
	if isDangerousProg(prog) {
		return "", fmt.Errorf("exec: program '%s' is disallowed", prog)
	}

	// When using an interpreter, relax argument validation:
	// - With -c: the code argument can contain any characters (it's source code).
	// - Without -c: the first argument is a script path — allow relative paths
	//   (containing /) as long as they don't escape with "..".
	interpreterMode := isInterpreter(prog)

	// Package managers (pip/pip3): allow all arguments through.
	// They need flags like --user, --break-system-packages and package names.
	pkgMgrMode := isPackageManager(prog)

	for i, a := range argv[1:] {
		idx := i + 1 // index in argv
		if pkgMgrMode {
			// Only reject directory traversal for safety
			if strings.Contains(a, "..") {
				return "", fmt.Errorf("exec: argument '%s' looks unsafe", a)
			}
			continue
		}
		if interpreterMode {
			// Skip validation for the code string after -c
			if idx >= 2 && len(argv) >= 3 && argv[1] == "-c" && idx == 2 {
				continue
			}
			// Allow script paths (first arg when not -c):
			// - Relative paths are allowed as-is (e.g. ./script.py, skills/monitor.py)
			// - Absolute paths inside the allowed workspace are converted to relative
			// - Reject directory traversal (..) in all cases
			if idx == 1 && a != "-c" && !strings.Contains(a, "..") {
				if strings.HasPrefix(a, "/") && t.allowedDir != "" {
					rel, err := filepath.Rel(t.allowedDir, a)
					if err == nil && !strings.HasPrefix(rel, "..") {
						argv[idx] = rel
					} else {
						return "", fmt.Errorf("exec: script path '%s' is outside workspace", a)
					}
				}
				continue
			}
		}
		if hasUnsafeArg(a) {
			return "", fmt.Errorf("exec: argument '%s' looks unsafe", a)
		}
	}

	cctx := ctx
	if t.timeout > 0 {
		var cancel context.CancelFunc
		cctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(cctx, prog, argv[1:]...)
	if t.allowedDir != "" {
		cmd.Dir = t.allowedDir
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("exec error: %w", err)
	}
	// Trim trailing newline for nicer test assertions
	out := string(b)
	out = strings.TrimRight(out, "\n")
	return out, nil
}
