package policy

import (
	"encoding/json"
	"fmt"

	"github.com/Builder106/halberd/internal/jsonrpc"
)

// Engine is the per-process policy evaluator. Engines are immutable after
// construction and safe for concurrent use across goroutines.
type Engine struct {
	bundle *Bundle
}

// New returns an Engine bound to b. The Bundle should already have been
// compiled by ParseBundle or LoadBundle; New does no further validation.
func New(b *Bundle) *Engine {
	return &Engine{bundle: b}
}

// Server returns the bundle's `server:` identifier, for use in audit logs.
func (e *Engine) Server() string { return e.bundle.Server }

// HasResponseFilters reports whether the bundle configures any response
// inspection. Transports use this to avoid buffering response bodies when
// no inspection would occur — a hot-path optimization for bundles that
// only do request-side enforcement.
func (e *Engine) HasResponseFilters() bool {
	return e.bundle.ResponseFilters != nil
}

// EvaluateRequest decides whether the JSON-RPC payload should be forwarded
// to the upstream MCP server. Returns a Decision with Blocked=false and no
// Violations on the happy path; the hot-path allocator profile in
// engine_test.go's benchmarks tracks this case explicitly.
func (e *Engine) EvaluateRequest(payload []byte) Decision {
	var env jsonrpc.Request
	if err := json.Unmarshal(payload, &env); err != nil {
		return Decision{
			Blocked: true,
			Violations: []Violation{{
				Category: CategoryMalformed,
				Rule:     "json_envelope",
				Detail:   err.Error(),
			}},
		}
	}

	switch env.Method {
	case "tools/call":
		return e.evaluateToolCall(env.Params)
	case "tools/list", "resources/list", "resources/read", "prompts/list", "prompts/get", "initialize", "ping":
		return Decision{}
	default:
		if e.bundle.Defaults.UnknownMethod == DispositionDeny {
			return Decision{
				Blocked: true,
				Violations: []Violation{{
					Category: CategoryOutOfScope,
					Rule:     "unknown_method",
					Detail:   fmt.Sprintf("method %q not in policy", env.Method),
				}},
			}
		}
		return Decision{}
	}
}

func (e *Engine) evaluateToolCall(raw json.RawMessage) Decision {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return Decision{
			Blocked: true,
			Violations: []Violation{{
				Category: CategoryMalformed,
				Rule:     "tool_params",
				Detail:   err.Error(),
			}},
		}
	}

	rule, found := e.bundle.toolRule(params.Name)
	if !found {
		if e.bundle.Defaults.UnknownTool == DispositionDeny {
			return Decision{
				Blocked: true,
				Violations: []Violation{{
					Category: CategoryCapabilityCreep,
					Tool:     params.Name,
					Rule:     "unknown_tool",
					Detail:   "tool not declared in policy bundle",
				}},
			}
		}
		return Decision{}
	}

	var violations []Violation
	for argName, argRule := range rule.Arguments {
		val, present := params.Arguments[argName]
		if !present {
			continue
		}
		violations = append(violations, evaluateArg(params.Name, argName, val, argRule)...)
	}

	return Decision{
		Blocked:    len(violations) > 0,
		Violations: violations,
	}
}

func evaluateArg(tool, name string, val any, rule ArgumentRule) []Violation {
	str, ok := val.(string)
	if !ok {
		if rule.Type == "string" {
			return []Violation{{
				Category: CategoryArgInjection,
				Tool:     tool,
				Field:    name,
				Rule:     "type_mismatch",
				Detail:   fmt.Sprintf("expected string, got %T", val),
			}}
		}
		return nil
	}

	var out []Violation

	if rule.MaxLength > 0 && len(str) > rule.MaxLength {
		out = append(out, Violation{
			Category: CategoryArgInjection,
			Tool:     tool,
			Field:    name,
			Rule:     "max_length",
			Detail:   fmt.Sprintf("argument length %d exceeds max %d", len(str), rule.MaxLength),
		})
	}

	if rule.allowSet != nil {
		if _, ok := rule.allowSet[str]; !ok {
			out = append(out, Violation{
				Category: CategoryArgInjection,
				Tool:     tool,
				Field:    name,
				Rule:     "allow_values",
				Detail:   fmt.Sprintf("value %q not in allowlist", truncate(str, 64)),
			})
		}
	}

	for _, re := range rule.denyCompiled {
		if loc := re.FindStringIndex(str); loc != nil {
			out = append(out, Violation{
				Category: CategoryArgInjection,
				Tool:     tool,
				Field:    name,
				Rule:     "deny_pattern",
				Detail:   fmt.Sprintf("matched %q at offset %d: %q", re.String(), loc[0], truncate(str[loc[0]:loc[1]], 64)),
			})
		}
	}

	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
