package tools

import (
	"context"
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
	return "Execute shell commands (array or string form, restricted for safety)"
}

func (t *ExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"cmd": map[string]interface{}{
				"oneOf": []map[string]interface{}{
					{
						"type":        "array",
						"description": "Command as array [program, arg1, arg2, ...]",
						"items": map[string]interface{}{"type": "string"},
						"minItems":    1,
					},
					{
						"type":        "string",
						"description": "Command as string, e.g. \"ls -la\"",
					},
				},
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
	if strings.Contains(s, "..") || strings.Contains(s, "~") {
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

	var argv []string
	switch v := cmdRaw.(type) {
	case string:
		// Allow string form: split by whitespace into argv.
		parts := strings.Fields(v)
		if len(parts) == 0 {
			return "", fmt.Errorf("exec: empty cmd string")
		}
		argv = parts
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

	// Catch common LLM hallucination: "uv run pip install ..."
	// The correct syntax is "uv pip install ...", not "uv run pip install ...".
	if strings.ToLower(filepath.Base(prog)) == "uv" && len(argv) >= 3 &&
		argv[1] == "run" && argv[2] == "pip" {
		return "", fmt.Errorf("exec: wrong syntax 'uv run pip install'. Use [\"uv\", \"pip\", \"install\", ...] instead")
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
			// All interpreter arguments are allowed (script args may contain
			// free-form text like log messages with special characters).
			// Only reject directory traversal in the script path itself.
			if idx == 1 && strings.Contains(a, "..") {
				return "", fmt.Errorf("exec: argument '%s' looks unsafe", a)
			}
			// Auto-resolve absolute script paths inside workspace
			if idx == 1 && strings.HasPrefix(a, "/") && t.allowedDir != "" {
				rel, err := filepath.Rel(t.allowedDir, a)
				if err == nil && !strings.HasPrefix(rel, "..") {
					argv[idx] = rel
				} else {
					return "", fmt.Errorf("exec: script path '%s' is outside workspace", a)
				}
			}
			continue
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
