package httpErrorHandler_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	httpErrorHandler "github.com/thesahibnanda-max/eCommerce/backend/package/handler/internal/error_handler"
)

var (
	errNotFound = errors.New("not found")
	errSpecific = fmt.Errorf("specific: %w", errNotFound)
	errUnknown  = errors.New("unknown")
)

func newHandler() httpErrorHandler.ErrorHandler {
	return httpErrorHandler.NewErrorHandler(
		httpErrorHandler.Rule{Err: errSpecific, StatusCode: http.StatusConflict, Code: "specific", Message: "specific"},
		httpErrorHandler.Rule{Err: errNotFound, StatusCode: http.StatusNotFound, Code: "not_found", Message: "not found"},
		httpErrorHandler.Rule{Err: nil, StatusCode: http.StatusTeapot, Code: "ignored"},
		httpErrorHandler.Rule{Err: errUnknown, StatusCode: 0, Code: "ignored"},
	)
}

func TestHandleError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantOK     bool
	}{
		{"nil", nil, 0, "", false},
		{"exact sentinel", errNotFound, http.StatusNotFound, "not_found", true},
		{"wrapped sentinel", fmt.Errorf("repo: %w", errNotFound), http.StatusNotFound, "not_found", true},
		{"first rule wins", errSpecific, http.StatusConflict, "specific", true},
		{"rule with zero status skipped", errUnknown, 0, "", false},
		{"unknown", errors.New("boom"), 0, "", false},
		{"typed error", httpErrorHandler.ErrQueryInvalid("limit", errNotFound), http.StatusBadRequest, "query_invalid", true},
		{"wrapped typed error", fmt.Errorf("ctx: %w", httpErrorHandler.ErrBodyInvalid(nil)), http.StatusBadRequest, "body_invalid", true},
		{"typed unauthorized", httpErrorHandler.ErrUnauthorizedHeader("Authorization"), http.StatusUnauthorized, "unauthorized", true},
		{"typed file required", httpErrorHandler.ErrFileRequired("file"), http.StatusBadRequest, "file_required", true},
	}
	h := newHandler()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for range 20 {
				status, resp, ok := h.HandleError(tc.err)
				if status != tc.wantStatus || resp.Code != tc.wantCode || ok != tc.wantOK {
					t.Fatalf("got (%d, %q, %v), want (%d, %q, %v)", status, resp.Code, ok, tc.wantStatus, tc.wantCode, tc.wantOK)
				}
			}
		})
	}
}

func TestErrorUnwrapAndMessage(t *testing.T) {
	e := httpErrorHandler.ErrQueryInvalid("limit", errNotFound)
	if !errors.Is(e, errNotFound) {
		t.Fatalf("typed error does not unwrap to its cause")
	}
	if e.Message != "query parameter limit is invalid" {
		t.Fatalf("message = %q", e.Message)
	}
	if got := httpErrorHandler.NewError(http.StatusTeapot, "tea", "short and stout", nil).Error(); got != "tea: short and stout" {
		t.Fatalf("Error() = %q", got)
	}
}
