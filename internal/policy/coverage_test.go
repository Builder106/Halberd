package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundle_LoadBundle(t *testing.T) {
	tmpDir := t.TempDir()
	bundlePath := filepath.Join(tmpDir, "policy.yaml")
	content := []byte(`
version: 1
server: test-server
tools:
  - name: echo
    arguments:
      msg:
        type: string
defaults:
  unknown_tool: deny
  unknown_method: log_and_pass
`)
	if err := os.WriteFile(bundlePath, content, 0600); err != nil {
		t.Fatalf("failed to write tmp bundle: %v", err)
	}

	b, err := LoadBundle(bundlePath)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}
	if b.Server != "test-server" {
		t.Errorf("expected test-server, got %s", b.Server)
	}

	_, err = LoadBundle(filepath.Join(tmpDir, "non-existent.yaml"))
	if err == nil {
		t.Fatal("expected error loading non-existent bundle")
	}
}

func TestBundle_ParseBundle_Errors(t *testing.T) {
	// Invalid YAML
	_, err := ParseBundle([]byte(`: invalid yaml`))
	if err == nil {
		t.Fatal("expected yaml parse error")
	}

	// Tool with empty name
	emptyToolYAML := []byte(`
version: 1
server: test
tools:
  - name: ""
`)
	_, err = ParseBundle(emptyToolYAML)
	if err == nil || !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("expected empty name error, got %v", err)
	}

	// Supported types check (number, boolean)
	typesYAML := []byte(`
version: 1
server: test
tools:
  - name: test_types
    arguments:
      num:
        type: number
      flag:
        type: boolean
`)
	b, err := ParseBundle(typesYAML)
	if err != nil {
		t.Fatalf("expected types valid, got %v", err)
	}
	if b == nil {
		t.Fatal("bundle should not be nil")
	}
}

func TestViolation_String(t *testing.T) {
	v1 := Violation{
		Category: CategoryArgInjection,
		Tool:     "bash",
		Field:    "cmd",
		Rule:     "deny_pattern",
		Detail:   "matched rm -rf",
	}
	str1 := v1.String()
	if !strings.Contains(str1, "tool=\"bash\" field=\"cmd\"") {
		t.Errorf("unexpected string: %s", str1)
	}

	v2 := Violation{
		Category: CategoryCapabilityCreep,
		Tool:     "eval",
		Rule:     "unknown_tool",
		Detail:   "tool not allowed",
	}
	str2 := v2.String()
	if !strings.Contains(str2, "tool=\"eval\" rule=\"unknown_tool\"") {
		t.Errorf("unexpected string: %s", str2)
	}

	v3 := Violation{
		Category: CategoryMalformed,
		Rule:     "json_envelope",
		Detail:   "invalid json",
	}
	str3 := v3.String()
	if !strings.Contains(str3, "rule=\"json_envelope\"") {
		t.Errorf("unexpected string: %s", str3)
	}
}

func TestEngine_Server_HasResponseFilters(t *testing.T) {
	b, err := ParseBundle([]byte(bundleWithResponseFilters))
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	e := New(b)
	if e.Server() != "test" {
		t.Errorf("Server() = %s, want test", e.Server())
	}
	if !e.HasResponseFilters() {
		t.Error("expected HasResponseFilters() == true")
	}

	bNo, err := ParseBundle([]byte(bundleNoResponseFilters))
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	eNo := New(bNo)
	if eNo.HasResponseFilters() {
		t.Error("expected HasResponseFilters() == false")
	}
}

func TestEngine_EvaluateRequest_UnknownMethods(t *testing.T) {
	// Unknown method with DispositionDeny
	const denyMethodSrc = `
version: 1
server: test
tools: []
defaults:
  unknown_tool: deny
  unknown_method: deny
`
	eDeny := newEngine(t, denyMethodSrc)
	d := eDeny.EvaluateRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"custom/method"}`))
	if !d.Blocked {
		t.Fatal("expected custom/method blocked under unknown_method: deny")
	}
	if len(d.Violations) == 0 || d.Violations[0].Category != CategoryOutOfScope {
		t.Errorf("expected out_of_scope violation, got %+v", d.Violations)
	}

	// Unknown method with DispositionAllow / log_and_pass
	const passMethodSrc = `
version: 1
server: test
tools: []
defaults:
  unknown_tool: deny
  unknown_method: log_and_pass
`
	ePass := newEngine(t, passMethodSrc)
	d2 := ePass.EvaluateRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"custom/method"}`))
	if d2.Blocked {
		t.Fatalf("expected pass for custom/method, got %v", d2.Violations)
	}

	// Other standard methods
	for _, m := range []string{"resources/list", "resources/read", "prompts/list", "prompts/get", "initialize", "ping"} {
		d3 := ePass.EvaluateRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"` + m + `"}`))
		if d3.Blocked {
			t.Errorf("method %s should be allowed, got %v", m, d3.Violations)
		}
	}
}

func TestEngine_EvaluateRequest_ToolParams_Errors(t *testing.T) {
	e := newEngine(t, postgresBundle)
	// Malformed params (e.g. integer instead of object)
	d := e.EvaluateRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":12345}`))
	if !d.Blocked {
		t.Fatal("expected malformed params blocked")
	}
	if len(d.Violations) == 0 || d.Violations[0].Category != CategoryMalformed {
		t.Errorf("expected malformed category, got %+v", d.Violations)
	}

	// Unknown tool with unknown_tool: allow
	const allowToolSrc = `
version: 1
server: test
tools: []
defaults:
  unknown_tool: allow
  unknown_method: log_and_pass
`
	eAllowTool := newEngine(t, allowToolSrc)
	dTool := eAllowTool.EvaluateRequest(req("tools/call", "any_tool", "param", "value"))
	if dTool.Blocked {
		t.Fatalf("expected allowed unknown tool, got %v", dTool.Violations)
	}
}

func TestEngine_EvaluateArg_TypesAndEdgeCases(t *testing.T) {
	const typeCheckSrc = `
version: 1
server: test
tools:
  - name: test_tool
    arguments:
      str_arg:
        type: string
        max_length: 10
        deny_patterns: ["bad"]
      num_arg:
        type: number
defaults:
  unknown_tool: deny
  unknown_method: log_and_pass
`
	e := newEngine(t, typeCheckSrc)

	// Non-string val when type is string
	dTypeMismatch := e.EvaluateRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":{"str_arg":12345}}}`))
	if !dTypeMismatch.Blocked {
		t.Fatal("expected type mismatch blocked")
	}
	if dTypeMismatch.Violations[0].Rule != "type_mismatch" {
		t.Errorf("expected type_mismatch rule, got %s", dTypeMismatch.Violations[0].Rule)
	}

	// Non-string val when type is number
	dNumOk := e.EvaluateRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":{"num_arg":12345}}}`))
	if dNumOk.Blocked {
		t.Fatalf("expected number allowed, got %v", dNumOk.Violations)
	}

	// Truncate long value in truncate() helper
	longBadStr := "this is a very long string containing bad pattern that exceeds length 64 bytes easily for truncation testing"
	dTrunc := e.EvaluateRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":{"str_arg":"` + longBadStr + `"}}}`))
	if !dTrunc.Blocked {
		t.Fatal("expected long bad string blocked")
	}
}

func TestTruncate(t *testing.T) {
	if s := truncate("short", 10); s != "short" {
		t.Errorf("expected 'short', got %q", s)
	}
	if s := truncate("longer string", 4); s != "long…" {
		t.Errorf("expected 'long…', got %q", s)
	}
}

func TestEngine_EvaluateResponse_EdgeCases(t *testing.T) {
	b, _ := ParseBundle([]byte(bundleWithResponseFilters))
	e := New(b)

	// Empty result
	inEmptyResult := []byte(`{"jsonrpc":"2.0","id":1}`)
	rEmpty := e.EvaluateResponse(inEmptyResult)
	if rEmpty.Modified {
		t.Error("expected no modification on empty result")
	}

	// Non-string leaf in result (e.g. number, boolean, null)
	inNumbers := []byte(`{"jsonrpc":"2.0","id":1,"result":{"count":42,"active":true,"data":null}}`)
	rNum := e.EvaluateResponse(inNumbers)
	if rNum.Modified {
		t.Error("expected no modification on primitives")
	}

	// Array of strings containing secret
	inArray := []byte(`{"jsonrpc":"2.0","id":1,"result":["clean", "AKIAIOSFODNN7EXAMPLE", 100]}`)
	rArr := e.EvaluateResponse(inArray)
	if !rArr.Modified {
		t.Error("expected array modification")
	}
	if !strings.Contains(string(rArr.Payload), "[REDACTED]") {
		t.Errorf("expected [REDACTED], got %s", rArr.Payload)
	}

	// Array without modifications
	inArrayClean := []byte(`{"jsonrpc":"2.0","id":1,"result":["clean1", "clean2"]}`)
	rArrClean := e.EvaluateResponse(inArrayClean)
	if rArrClean.Modified {
		t.Error("expected no modification on clean array")
	}
}

func TestGlobalResponseFilter_UnknownScanner(t *testing.T) {
	// Directly construct filter with unknown scanner name
	f := &GlobalResponseFilter{
		SecretScanners: []string{"unknown_scanner"},
	}
	out, det := f.sanitizeString("test string", "")
	if out != "test string" || len(det) != 0 {
		t.Errorf("expected passthrough for unknown scanner, got %s, %v", out, det)
	}
}
