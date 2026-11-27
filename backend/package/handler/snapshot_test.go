package handler_test

import (
	"net/http"
	"testing"

	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
)

func TestAdminSnapshot_NotConfigured(t *testing.T) {
	f := newFixture(t)
	r := f.do(t, request{method: http.MethodGet, path: "/v1/admin/snapshot", adminKey: testAdminKey})
	if r.status != http.StatusOK {
		t.Fatalf("status = %d body %s", r.status, r.body)
	}
	if status := decode[adminOperationsService.SnapshotResponse](t, r); status.Configured {
		t.Fatalf("status = %+v", status)
	}
	expectError(t, f.do(t, request{method: http.MethodPost, path: "/v1/admin/snapshot", adminKey: testAdminKey}), http.StatusServiceUnavailable, "snapshot_not_configured")
	expectError(t, f.do(t, request{method: http.MethodPost, path: "/v1/admin/snapshot", adminKey: "wrong"}), http.StatusUnauthorized, "admin_unauthorized")
	if r := f.do(t, request{method: http.MethodGet, path: "/v1/admin/snapshot"}); r.status != http.StatusUnauthorized {
		t.Fatalf("missing key status = %d", r.status)
	}
}
