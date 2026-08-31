package jsonrpc

import (
	"encoding/json"
)

// PolicyViolation returns the bytes of a JSON-RPC error response for a request
// that Halberd blocked. id may be nil for notifications, in which case the
// response is suppressed by the caller.
func PolicyViolation(id json.RawMessage, summary string, violations any) ([]byte, error) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    CodePolicyViolation,
			Message: summary,
			Data:    violations,
		},
	}
	return json.Marshal(resp)
}
