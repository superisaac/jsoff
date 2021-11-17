package jsonrpc

// https://www.jsonrpc.org/specification

var (
	ErrServerError = &RPCError{100, "server error", nil}
	ErrNilId       = &RPCError{102, "nil message id", nil}

	ErrMethodNotFound = &RPCError{-32601, "method not found", nil}
	ErrEmptyMethod    = &RPCError{-32601, "empty method", nil}

	ErrParseMessage = &RPCError{-32700, "parse error", nil}
	ErrNotAllowed   = &RPCError{-32503, "type not allowed", nil}

	ErrMessageType = &RPCError{105, "wrong message type", nil}

	ErrTimeout     = &RPCError{200, "request timeout", nil}
	ErrBadResource = &RPCError{201, "bad resource", nil}
	ErrLiveExit    = &RPCError{202, "live exit", nil}

	ErrAuthFailed = &RPCError{401, "auth failed", nil}
)

func ParamsError(message string) *RPCError {
	return &RPCError{400, message, nil}
}
