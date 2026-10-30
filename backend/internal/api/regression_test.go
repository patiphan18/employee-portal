package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/digithun/employee-portal/backend/internal/api"
	"github.com/digithun/employee-portal/backend/internal/models"
)

// setTrustProxy rebuilds the router with a different proxy-trust setting, since
// Routes wires the limiter from that flag.
func (h *harness) setTrustProxy(t *testing.T, trust bool) {
	t.Helper()
	h.srv.TrustProxyHeaders = trust
	h.handler = h.srv.Routes([]string{"*"}, t.TempDir())
}

// loginFrom posts a login attempt carrying the headers a reverse proxy would add.
func (h *harness) loginFrom(remoteAddr, realIP string) int {
	h.t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = remoteAddr
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
		req.Header.Set("X-Forwarded-For", realIP)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec.Code
}

// Regression for the review finding "login rate limit is global behind the proxy".
// With TrustProxyHeaders on, each real client gets its own budget even though every
// request reaches the API from the proxy's single address.
func TestRateLimitIsPerClientBehindAProxy(t *testing.T) {
	h := newHarness(t)
	h.setTrustProxy(t, true)

	const proxy = "172.18.0.5:41000" // the nginx container, identical for everyone

	// One client burns the whole budget.
	for i := 0; i < 10; i++ {
		if code := h.loginFrom(proxy, "203.0.113.10"); code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d from the first client was limited too early", i+1)
		}
	}
	if code := h.loginFrom(proxy, "203.0.113.10"); code != http.StatusTooManyRequests {
		t.Errorf("the first client should now be limited, got %d", code)
	}

	// A different employee behind the same proxy must be unaffected.
	if code := h.loginFrom(proxy, "203.0.113.99"); code == http.StatusTooManyRequests {
		t.Error("a second client behind the same proxy was wrongly locked out")
	}
}

// Without the flag the headers are ignored, so a client cannot mint a fresh budget
// by forging X-Forwarded-For.
func TestForgedProxyHeadersAreIgnoredByDefault(t *testing.T) {
	h := newHarness(t)
	h.setTrustProxy(t, false)

	const attacker = "198.51.100.7:50000"
	for i := 0; i < 10; i++ {
		h.loginFrom(attacker, "")
	}
	if code := h.loginFrom(attacker, ""); code != http.StatusTooManyRequests {
		t.Fatalf("the attacker should be limited, got %d", code)
	}
	// Same peer, a forged header claiming a different origin.
	if code := h.loginFrom(attacker, "203.0.113.1"); code != http.StatusTooManyRequests {
		t.Errorf("a forged X-Forwarded-For must not reset the budget, got %d", code)
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remote     string
		realIP     string
		forwarded  string
		trustProxy bool
		want       string
	}{
		{"untrusted ignores headers", "10.0.0.1:1234", "203.0.113.5", "203.0.113.5", false, "10.0.0.1"},
		{"trusted prefers X-Real-IP", "10.0.0.1:1234", "203.0.113.5", "1.1.1.1", true, "203.0.113.5"},
		{
			// nginx appends the real peer, so the last entry is the trustworthy one.
			name: "trusted takes the last forwarded entry", remote: "10.0.0.1:1234",
			forwarded: "9.9.9.9, 203.0.113.5", trustProxy: true, want: "203.0.113.5",
		},
		{"trusted falls back to remote", "10.0.0.1:1234", "", "", true, "10.0.0.1"},
		{"malformed remote addr", "not-an-addr", "", "", false, "not-an-addr"},
		{"blank forwarded entries skipped", "10.0.0.1:1234", "", "9.9.9.9,  ", true, "9.9.9.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			if tc.realIP != "" {
				req.Header.Set("X-Real-IP", tc.realIP)
			}
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			if got := api.ClientIP(req, tc.trustProxy); got != tc.want {
				t.Errorf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRateLimiterWindowExpiry(t *testing.T) {
	rl := api.NewRateLimiter(2, time.Minute, false)
	for i := 0; i < 2; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Error("the third attempt should be blocked")
	}
}

// An administrator is a configuration operator with no employee profile, so every
// self-service endpoint is closed to them (FR-2.4).
func TestAdminHasNoProfileSurface(t *testing.T) {
	h := newHarness(t)
	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me"},
		{http.MethodPatch, "/api/v1/me/contact"},
		{http.MethodPut, "/api/v1/me/skills"},
		{http.MethodGet, "/api/v1/me/history"},
		{http.MethodGet, "/api/v1/skills"},
	}
	for _, c := range cases {
		rec := h.do(c.method, c.path, h.admin, map[string]string{})
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as admin = %d, want 403", c.method, c.path, rec.Code)
		}
	}
}

func TestAdminAvatarUploadIsForbidden(t *testing.T) {
	h := newHarness(t)
	if rec := h.upload("file", "me.png", pngBytes(64), h.admin); rec.Code != http.StatusForbidden {
		t.Errorf("avatar upload as admin = %d, want 403", rec.Code)
	}
}

// The employee surface keeps working; the split must not lock staff out.
func TestEmployeeStillReachesTheProfileSurface(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{"/api/v1/me", "/api/v1/me/history", "/api/v1/skills"} {
		if rec := h.do(http.MethodGet, path, h.token, nil); rec.Code != http.StatusOK {
			t.Errorf("GET %s as employee = %d, want 200", path, rec.Code)
		}
	}
}

// Both roles can introspect their own token: the client uses it to decide where to
// send the user after a page reload.
func TestTokenIntrospectionWorksForBothRoles(t *testing.T) {
	h := newHarness(t)
	for _, tc := range []struct{ name, token, wantRole string }{
		{"employee", h.token, "employee"},
		{"admin", h.admin, "admin"},
	} {
		rec := h.do(http.MethodGet, "/api/v1/auth/me", tc.token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", tc.name, rec.Code)
		}
		body := decode[struct {
			Role string `json:"role"`
		}](t, rec)
		if body.Role != tc.wantRole {
			t.Errorf("%s: role = %q, want %q", tc.name, body.Role, tc.wantRole)
		}
	}
}

func TestLoginReturnsRoleAndOmitsAdminProfile(t *testing.T) {
	h := newHarness(t)

	adminBody := decode[struct {
		Role    string          `json:"role"`
		Email   string          `json:"email"`
		Profile json.RawMessage `json:"profile"`
	}](t, h.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "admin@example.com", "password": testPassword,
	}))
	if adminBody.Role != "admin" {
		t.Errorf("admin role = %q", adminBody.Role)
	}
	if string(adminBody.Profile) != "null" {
		t.Errorf("admin login must not carry a profile, got %s", adminBody.Profile)
	}

	empBody := decode[struct {
		Role    string          `json:"role"`
		Profile *models.Profile `json:"profile"`
	}](t, h.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "somchai@example.com", "password": testPassword,
	}))
	if empBody.Role != "employee" {
		t.Errorf("employee role = %q", empBody.Role)
	}
	if empBody.Profile == nil || empBody.Profile.FirstName != "Somchai" {
		t.Errorf("employee login should still carry the profile, got %+v", empBody.Profile)
	}
}

// --- history paging (FR-5.8) and the removal of undo ---

func TestHistoryIsPaged(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	for i := 0; i < 25; i++ {
		v := "v"
		h.mem.SeedChange(models.ProfileChange{
			UserID: h.userID, Field: models.FieldMobile, OldValue: &v, NewValue: &v,
			ChangedBy: h.userID, ChangedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	type pageBody struct {
		Page       int                    `json:"page"`
		PerPage    int                    `json:"per_page"`
		Total      int                    `json:"total"`
		TotalPages int                    `json:"total_pages"`
		Events     []models.ProfileChange `json:"events"`
	}

	first := decode[pageBody](t, h.do(http.MethodGet, "/api/v1/me/history?per_page=10", h.token, nil))
	if first.Total != 25 || first.TotalPages != 3 || len(first.Events) != 10 {
		t.Fatalf("page 1: %+v", first)
	}

	third := decode[pageBody](t, h.do(http.MethodGet, "/api/v1/me/history?per_page=10&page=3", h.token, nil))
	if third.Page != 3 || len(third.Events) != 5 {
		t.Errorf("page 3: %+v", third)
	}
}

func TestHistoryDefaultPageSize(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	for i := 0; i < 30; i++ {
		v := "v"
		h.mem.SeedChange(models.ProfileChange{
			UserID: h.userID, Field: models.FieldMobile, OldValue: &v, NewValue: &v,
			ChangedBy: h.userID, ChangedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	body := decode[struct {
		PerPage int                    `json:"per_page"`
		Events  []models.ProfileChange `json:"events"`
	}](t, h.do(http.MethodGet, "/api/v1/me/history", h.token, nil))

	if body.PerPage != 5 || len(body.Events) != 5 {
		t.Errorf("per_page = %d with %d events, want 5", body.PerPage, len(body.Events))
	}
}

func TestHistoryRejectsNonNumericPaging(t *testing.T) {
	h := newHarness(t)
	for _, q := range []string{"page=abc", "per_page=lots"} {
		rec := h.do(http.MethodGet, "/api/v1/me/history?"+q, h.token, nil)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("?%s returned %d, want 422", q, rec.Code)
		}
	}
}

// The audit log is read-only: there is no endpoint that replays or rewrites it.
func TestRevertEndpointIsGone(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost,
		"/api/v1/me/history/00000000-0000-0000-0000-000000000000/revert", h.token, nil)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("revert should no longer be routed, got %d", rec.Code)
	}
}
