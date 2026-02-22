package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecArrayEcho(t *testing.T) {
	e := NewExecTool(2)
	out, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"echo", "hello"}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != "hello" {
		t.Fatalf("unexpected out: %s", out)
	}
}

func TestExecStringDisallowed(t *testing.T) {
	e := NewExecTool(2)
	_, err := e.Execute(context.Background(), map[string]interface{}{"cmd": "ls -la"})
	if err == nil {
		t.Fatalf("expected error for string command")
	}
}

func TestExecDangerousProgRejected(t *testing.T) {
	e := NewExecTool(2)
	_, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"rm", "-rf", "/"}})
	if err == nil {
		t.Fatalf("expected error for dangerous program")
	}
}

func TestExecWithWorkspace(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "file.txt")
	os.WriteFile(f, []byte("content"), 0644)
	e := NewExecToolWithWorkspace(2, d)
	out, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"cat", "file.txt"}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != "content" {
		t.Fatalf("unexpected out: %s", out)
	}
}

func TestExecRejectsUnsafeArg(t *testing.T) {
	e := NewExecTool(2)
	_, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"ls", "/etc"}})
	if err == nil {
		t.Fatalf("expected error for absolute path arg")
	}
}

func TestExecPipAllowed(t *testing.T) {
	e := NewExecTool(5)
	// pip3 with --user and package name should be allowed (args contain - and flags)
	_, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"pip3", "install", "--user", "requests"}})
	// We don't care if pip3 is actually installed; we only check that it wasn't rejected by the sandbox.
	if err != nil && (err.Error() == "exec: argument '--user' looks unsafe" || err.Error() == "exec: argument 'install' looks unsafe") {
		t.Fatalf("pip3 arguments should not be rejected by sandbox: %v", err)
	}
}

func TestExecUvAllowed(t *testing.T) {
	e := NewExecTool(5)
	// uv with pip install --system should not be rejected by sandbox
	_, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"uv", "pip", "install", "--system", "requests"}})
	if err != nil && strings.Contains(err.Error(), "looks unsafe") {
		t.Fatalf("uv arguments should not be rejected by sandbox: %v", err)
	}
}

func TestExecUvVenvAllowed(t *testing.T) {
	e := NewExecTool(5)
	// uv venv with a path containing / should be allowed for package managers
	_, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"uv", "venv", "venvs/my-project"}})
	if err != nil && strings.Contains(err.Error(), "looks unsafe") {
		t.Fatalf("uv venv path should not be rejected by sandbox: %v", err)
	}
}

func TestExecPipRejectsTraversal(t *testing.T) {
	e := NewExecTool(2)
	_, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"pip3", "install", "--target", "../escape"}})
	if err == nil {
		t.Fatalf("expected error for directory traversal in pip args")
	}
}

func TestExecTimeout(t *testing.T) {
	e := NewExecTool(1)
	_, err := e.Execute(context.Background(), map[string]interface{}{"cmd": []interface{}{"sleep", "2"}})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
}
