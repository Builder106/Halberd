// Package jsonrpc implements the subset of JSON-RPC 2.0 used by the Model Context
// Protocol. The MCP spec is JSON-RPC 2.0 over either stdio or HTTP+SSE; this
// package only models the envelope and the error-response shape Halberd needs to
// synthesize when blocking a request.
package jsonrpc

import (
	"encoding/json"
)

// Request is a JSON-RPC 2.0 request envelope. ID is left as a RawMessage so
// that whatever encoding (number or string) the client used round-trips back
// verbatim in the corresponding Response.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response envelope. Either Result or Error is
// set, never both, per the spec.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is the JSON-RPC 2.0 error object embedded in a Response.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}


// JSON-RPC 2.0 reserved error codes plus Halberd's policy-violation code.
// CodePolicyViolation sits in the -32000..-32099 server-defined range.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603

	CodePolicyViolation = -32000
)
