package jsonz

import ()

// error defination https://www.jsonrpc.org/specification#error_object
var (
	ErrServerError = &RPCError{100, "server error", nil}
	ErrNilId       = &RPCError{102, "nil message id", nil}

	ErrMethodNotFound = &RPCError{-32601, "method not found", nil}

	ErrEmptyMethod = &RPCError{-32601, "empty method", nil}

	ErrParseMessage   = &RPCError{-32700, "parse error", nil}
	ErrInvalidRequest = &RPCError{-32600, "invalid request", nil}

	ErrInternalError = &RPCError{-32603, "internal error", nil}

	ErrMessageType = &RPCError{105, "wrong message type", nil}

	ErrTimeout     = &RPCError{200, "request timeout", nil}
	ErrBadResource = &RPCError{201, "bad resource", nil}
	ErrLiveExit    = &RPCError{202, "live exit", nil}

	ErrNotAllowed = &RPCError{406, "type not allowed", nil}
	ErrAuthFailed = &RPCError{401, "auth failed", nil}

	ErrInvalidSchema = &RPCError{-32633, "invalid schema", nil}
)

func ParamsError(message string) *RPCError {
	return &RPCError{-32602, message, nil}
}
