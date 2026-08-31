package policy

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
)

// packsDir returns the absolute path to the policies/ directory in the
// repository. Built from the test file's own path so it works regardless
// of which package directory `go test` is invoked from.
func packsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "policies")
}

type packScenario struct {
	name    string
	tool    string
	args    map[string]any
	blocked bool
}

func runPackScenarios(t *testing.T, bundlePath string, scenarios []packScenario) {
	t.Helper()
	b, err := LoadBundle(bundlePath)
	if err != nil {
		t.Fatalf("load %s: %v", bundlePath, err)
	}
	e := New(b)

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "tools/call",
				"params": map[string]any{
					"name":      sc.tool,
					"arguments": sc.args,
				},
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			d := e.EvaluateRequest(payload)
			if d.Blocked != sc.blocked {
				t.Fatalf("blocked = %v, want %v; violations: %+v", d.Blocked, sc.blocked, d.Violations)
			}
		})
	}
}

func TestPack_Filesystem(t *testing.T) {
	runPackScenarios(t, filepath.Join(packsDir(t), "mcp-server-filesystem.yaml"), []packScenario{
		{
			name:    "allow_relative_read",
			tool:    "read_file",
			args:    map[string]any{"path": "src/main.go"},
			blocked: false,
		},
		{
			name:    "block_absolute_path",
			tool:    "read_file",
			args:    map[string]any{"path": "/etc/passwd"},
			blocked: true,
		},
		{
			name:    "block_path_traversal",
			tool:    "read_file",
			args:    map[string]any{"path": "../../etc/shadow"},
			blocked: true,
		},
		{
			name:    "block_home_expansion",
			tool:    "read_file",
			args:    map[string]any{"path": "~/.ssh/id_rsa"},
			blocked: true,
		},
		{
			name:    "block_null_byte",
			tool:    "read_file",
			args:    map[string]any{"path": "ok.txt\x00.png"},
			blocked: true,
		},
		{
			name:    "block_array_arg_tool",
			tool:    "read_multiple_files", // intentionally undeclared
			args:    map[string]any{"paths": []string{"a.txt"}},
			blocked: true,
		},
	})
}

func TestPack_Git(t *testing.T) {
	runPackScenarios(t, filepath.Join(packsDir(t), "mcp-server-git.yaml"), []packScenario{
		{
			name:    "allow_status",
			tool:    "git_status",
			args:    map[string]any{"repo_path": "."},
			blocked: false,
		},
		{
			name:    "allow_log_on_branch",
			tool:    "git_log",
			args:    map[string]any{"repo_path": "repo", "max_count": 10},
			blocked: false,
		},
		{
			name:    "block_long_opt_smuggling",
			tool:    "git_diff",
			args:    map[string]any{"repo_path": ".", "target": "--upload-pack=ssh://attacker/x"},
			blocked: true,
		},
		{
			name:    "block_ref_with_whitespace",
			tool:    "git_checkout",
			args:    map[string]any{"repo_path": ".", "branch_name": "main; rm -rf /"},
			blocked: true,
		},
		{
			name:    "block_state_mutating_commit",
			tool:    "git_commit", // intentionally undeclared
			args:    map[string]any{"repo_path": ".", "message": "x"},
			blocked: true,
		},
	})
}

func TestPack_GitHub(t *testing.T) {
	runPackScenarios(t, filepath.Join(packsDir(t), "mcp-server-github.yaml"), []packScenario{
		{
			name:    "allow_get_issue_in_org",
			tool:    "get_issue",
			args:    map[string]any{"owner": "your-org", "repo": "halberd", "issue_number": 42},
			blocked: false,
		},
		{
			name:    "block_owner_outside_allowlist",
			tool:    "get_issue",
			args:    map[string]any{"owner": "other-org", "repo": "halberd", "issue_number": 42},
			blocked: true,
		},
		{
			name:    "block_repo_with_shell_metachar",
			tool:    "get_issue",
			args:    map[string]any{"owner": "your-org", "repo": "halberd; rm -rf /", "issue_number": 1},
			blocked: true,
		},
		{
			name:    "block_invalid_state",
			tool:    "list_issues",
			args:    map[string]any{"owner": "your-org", "repo": "halberd", "state": "deleted"},
			blocked: true,
		},
		{
			name:    "block_dangerous_unknown_tool",
			tool:    "delete_repository", // intentionally undeclared
			args:    map[string]any{"owner": "your-org", "repo": "halberd"},
			blocked: true,
		},
	})
}

func TestPack_Postgres(t *testing.T) {
	// Sanity check that the existing pack still loads and behaves as
	// described in the README after the response_filters addition.
	runPackScenarios(t, filepath.Join(packsDir(t), "mcp-server-postgres.yaml"), []packScenario{
		{
			name:    "allow_simple_select",
			tool:    "query",
			args:    map[string]any{"sql": "SELECT id, name FROM students LIMIT 10"},
			blocked: false,
		},
		{
			name:    "block_drop_table",
			tool:    "query",
			args:    map[string]any{"sql": "DROP TABLE users"},
			blocked: true,
		},
		{
			name:    "block_pg_read_server_files",
			tool:    "query",
			args:    map[string]any{"sql": "SELECT pg_read_server_files('/etc/passwd')"},
			blocked: true,
		},
	})
}
