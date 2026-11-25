package httpErrorHandler

import (
	"net/http"
	"strconv"
)

const (
	codeBodyInvalid  = "body_invalid"
	codeQueryInvalid = "query_invalid"
	codeFileRequired = "file_required"
	codeFileInvalid  = "file_invalid"
	codeUnauthorized = "unauthorized"
	codeRateLimited  = "rate_limited"
)

func NewError(statusCode int, code, message string, cause error) *Error {
	return &Error{StatusCode: statusCode, Code: code, Message: message, Err: cause}
}

func BadRequest(code, message string, cause error) *Error {
	return NewError(http.StatusBadRequest, code, message, cause)
}

func ErrBodyInvalid(cause error) *Error {
	return BadRequest(codeBodyInvalid, "request body is not valid json", cause)
}

func ErrQueryInvalid(param string, cause error) *Error {
	return BadRequest(codeQueryInvalid, "query parameter "+param+" is invalid", cause)
}

func ErrFileRequired(field string) *Error {
	return BadRequest(codeFileRequired, "multipart field "+field+" is required", nil)
}

func ErrFileInvalid(cause error) *Error {
	return BadRequest(codeFileInvalid, "uploaded file could not be read", cause)
}

func ErrUnauthorizedHeader(header string) *Error {
	return NewError(http.StatusUnauthorized, codeUnauthorized, header+" header is missing", nil)
}

func ErrRateLimited(retryAfterSeconds int) *Error {
	return NewError(http.StatusTooManyRequests, codeRateLimited, "too many requests, retry in "+strconv.Itoa(retryAfterSeconds)+"s", nil)
}
