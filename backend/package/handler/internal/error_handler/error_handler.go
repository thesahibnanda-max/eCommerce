package httpErrorHandler

import "errors"

type Rule struct {
	Err        error
	StatusCode int
	Code       string
	Message    string
}

type Response struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Error struct {
	StatusCode int
	Code       string
	Message    string
	Err        error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

type ErrorHandler interface {
	HandleError(err error) (statusCode int, response Response, isHandled bool)
}

type errorHandler struct {
	rules []Rule
}

func NewErrorHandler(rules ...Rule) ErrorHandler {
	return &errorHandler{rules: validRules(rules)}
}

func (eh *errorHandler) HandleError(err error) (statusCode int, response Response, isHandled bool) {
	if err == nil {
		return 0, Response{}, false
	}
	if typed, ok := errors.AsType[*Error](err); ok {
		return typed.StatusCode, Response{Code: typed.Code, Message: typed.Message}, true
	}
	if rule, ok := eh.match(err); ok {
		return rule.StatusCode, Response{Code: rule.Code, Message: rule.Message}, true
	}
	return 0, Response{}, false
}
