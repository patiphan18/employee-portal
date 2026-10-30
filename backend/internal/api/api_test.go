package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/digithun/employee-portal/backend/internal/api"
	"github.com/digithun/employee-portal/backend/internal/auth"
	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/service"
	"github.com/digithun/employee-portal/backend/internal/store"
)

const testPassword = "Password123!"

func ptr(s string) *string { return &s }

// fakeStorage records what was saved without touching the filesystem.
type fakeStorage struct {
	saved   []byte
	removed []string
	present map[string]bool
	url     string
	err     error
}

func (f *fakeStorage) Save(r io.Reader) (string, error) {
	b, _ := io.ReadAll(r)
	f.saved = b
	if f.err != nil {
		return "", f.err
	}
	if f.url == "" {
		f.url = "/uploads/new.png"
	}
	if f.present == nil {
		f.present = map[string]bool{}
	}
	f.present[f.url] = true
	return f.url, nil
}

func (f *fakeStorage) Remove(url string) error {
	f.removed = append(f.removed, url)
	delete(f.present, url)
	return nil
}

func (f *fakeStorage) Exists(url string) bool { return f.present[url] }

type harness struct {
	t       *testing.T
	srv     *api.Server
	handler http.Handler
	mem     *store.Memory
	files   *fakeStorage
	token   string
	admin   string
	userID  string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	mem := store.NewMemory()
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	mem.SeedUser(&models.User{
		ID: "user-1", Email: "somchai@example.com", PasswordHash: hash,
		FirstName: "Somchai", LastName: "Jaidee", Mobile: ptr("0812345678"),
		Role: models.RoleEmployee,
	})
	mem.SeedUser(&models.User{
		ID: "user-2", Email: "admin@example.com", PasswordHash: hash,
		FirstName: "Araya", LastName: "Suksan", Role: models.RoleAdmin,
	})
	mem.SeedSkill(&models.Skill{ID: "s-go", Name: "Go", Active: true, SortOrder: 10})
	mem.SeedSkill(&models.Skill{ID: "s-react", Name: "React", Active: true, SortOrder: 20})
	mem.SeedSkill(&models.Skill{ID: "s-old", Name: "Flash", Active: false, SortOrder: 30})

	issuer := auth.NewIssuer("test-secret", time.Hour)
	profileSvc := service.NewProfile(mem)
	files := &fakeStorage{}
	srv := &api.Server{
		Store:          mem,
		Issuer:         issuer,
		Profile:        profileSvc,
		History:        service.NewHistory(mem, 7),
		Admin:          service.NewAdmin(mem, 7),
		Storage:        files,
		MaxUploadBytes: 2 * 1024 * 1024,
	}

	userToken, _, _ := issuer.Issue("user-1", "somchai@example.com", models.RoleEmployee)
	adminToken, _, _ := issuer.Issue("user-2", "admin@example.com", models.RoleAdmin)

	return &harness{
		t:       t,
		srv:     srv,
		handler: srv.Routes([]string{"*"}, t.TempDir()),
		mem:     mem,
		files:   files,
		token:   userToken,
		admin:   adminToken,
		userID:  "user-1",
	}
}

func (h *harness) do(method, path, token string, body any) *httptest.ResponseRecorder {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return out
}

// ---------- auth ----------

func TestLoginSuccess(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "somchai@example.com", "password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	body := decode[struct {
		Token   string         `json:"token"`
		Profile models.Profile `json:"profile"`
	}](t, rec)
	if body.Token == "" {
		t.Error("login should return a token")
	}
	if body.Profile.FirstName != "Somchai" {
		t.Errorf("profile not returned with login: %+v", body.Profile)
	}
	if strings.Contains(rec.Body.String(), "password_hash") {
		t.Error("the response must never contain the password hash")
	}
}

func TestLoginIsCaseInsensitiveOnEmail(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "SomChai@Example.COM", "password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	h := newHarness(t)
	unknown := h.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "nobody@example.com", "password": testPassword,
	})
	wrongPass := h.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "somchai@example.com", "password": "wrong",
	})
	if unknown.Code != http.StatusUnauthorized || wrongPass.Code != http.StatusUnauthorized {
		t.Fatalf("statuses = %d / %d, want 401 for both", unknown.Code, wrongPass.Code)
	}
	if unknown.Body.String() != wrongPass.Body.String() {
		t.Errorf("the two failures must look identical:\n%s\n%s", unknown.Body, wrongPass.Body)
	}
}

func TestLoginRateLimit(t *testing.T) {
	h := newHarness(t)
	var last int
	for i := 0; i < 12; i++ {
		last = h.do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
			"email": "somchai@example.com", "password": "wrong",
		}).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("the 12th attempt returned %d, want 429", last)
	}
}

func TestProtectedRoutesRequireAToken(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{"/api/v1/me", "/api/v1/me/history", "/api/v1/skills"} {
		if rec := h.do(http.MethodGet, path, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without a token = %d, want 401", path, rec.Code)
		}
	}
}

func TestInvalidTokenIsRejected(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(http.MethodGet, "/api/v1/me", "garbage.token.here", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// ---------- profile ----------

func TestGetProfile(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/v1/me", h.token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	p := decode[models.Profile](t, rec)
	if p.ID != "user-1" || p.Email != "somchai@example.com" {
		t.Errorf("wrong profile returned: %+v", p)
	}
	if p.Completeness.Percent != 40 {
		t.Errorf("completeness = %d, want 40 (email + mobile)", p.Completeness.Percent)
	}
}

func TestTokenDecidesWhoseProfileIsReturned(t *testing.T) {
	h := newHarness(t)
	// Even with another user's id in the query, the token wins (no IDOR).
	rec := h.do(http.MethodGet, "/api/v1/me?user_id=user-2", h.token, nil)
	p := decode[models.Profile](t, rec)
	if p.ID != "user-1" {
		t.Errorf("returned profile %q, want user-1", p.ID)
	}
}

func TestPatchContactSingleField(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPatch, "/api/v1/me/contact", h.token, map[string]string{
		"address": "99 Sukhumvit Rd, Bangkok",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	p := decode[models.Profile](t, rec)
	if p.Address == nil || *p.Address != "99 Sukhumvit Rd, Bangkok" {
		t.Errorf("address = %v", p.Address)
	}
	if p.Mobile == nil || *p.Mobile != "0812345678" {
		t.Errorf("mobile should be untouched, got %v", p.Mobile)
	}
}

func TestPatchContactIgnoresNameFields(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPatch, "/api/v1/me/contact", h.token, map[string]string{
		"first_name": "Hacker",
		"last_name":  "McHackface",
		"mobile":     "0899999999",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	p := decode[models.Profile](t, rec)
	if p.FirstName != "Somchai" || p.LastName != "Jaidee" {
		t.Errorf("names are read-only but changed to %q %q", p.FirstName, p.LastName)
	}
	if p.Mobile == nil || *p.Mobile != "0899999999" {
		t.Errorf("the legitimate part of the patch should still apply, got %v", p.Mobile)
	}
}

func TestPatchContactValidationReturns422(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPatch, "/api/v1/me/contact", h.token, map[string]string{
		"mobile": "not-a-phone",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := decode[api.ErrorBody](t, rec)
	if body.Errors["mobile"] == "" {
		t.Errorf("expected a per-field error, got %+v", body)
	}
}

func TestPatchContactEmptyBodyIs400(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(http.MethodPatch, "/api/v1/me/contact", h.token, map[string]string{}); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPatchContactDuplicateEmailIs409(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPatch, "/api/v1/me/contact", h.token, map[string]string{
		"email": "admin@example.com",
	})
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

// ---------- skills ----------

func TestListSkillsHidesInactive(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/v1/skills", h.token, nil)
	body := decode[struct {
		Skills []models.Skill `json:"skills"`
	}](t, rec)
	for _, s := range body.Skills {
		if !s.Active {
			t.Errorf("the employee catalog leaked an inactive skill: %+v", s)
		}
	}
	if len(body.Skills) != 2 {
		t.Errorf("got %d active skills, want 2", len(body.Skills))
	}
}

func TestPutSkills(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPut, "/api/v1/me/skills", h.token, map[string][]string{
		"skill_ids": {"s-go", "s-react"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	p := decode[models.Profile](t, rec)
	if len(p.Skills) != 2 {
		t.Errorf("got %d skills, want 2", len(p.Skills))
	}
	if p.Completeness.Percent != 60 {
		t.Errorf("completeness = %d, want 60", p.Completeness.Percent)
	}
}

func TestPutSkillsRejectsInactive(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPut, "/api/v1/me/skills", h.token, map[string][]string{
		"skill_ids": {"s-old"},
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

// ---------- avatar ----------

// pngBytes returns a minimal valid PNG header plus padding, which is enough for
// http.DetectContentType.
func pngBytes(size int) []byte {
	b := make([]byte, size)
	copy(b, []byte("\x89PNG\r\n\x1a\n"))
	return b
}

func (h *harness) upload(fieldName, filename string, content []byte, token string) *httptest.ResponseRecorder {
	h.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(fieldName, filename)
	if err != nil {
		h.t.Fatalf("CreateFormFile: %v", err)
	}
	part.Write(content)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func TestUploadAvatar(t *testing.T) {
	h := newHarness(t)
	rec := h.upload("file", "me.png", pngBytes(1024), h.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	p := decode[models.Profile](t, rec)
	if p.AvatarURL == nil || *p.AvatarURL != "/uploads/new.png" {
		t.Errorf("avatar_url = %v", p.AvatarURL)
	}
	if len(h.files.saved) != 1024 {
		t.Errorf("stored %d bytes, want 1024", len(h.files.saved))
	}

	events, _ := h.mem.History(nil, h.userID, time.Time{}, 0, 0)
	if len(events) != 1 || events[0].Field != models.FieldAvatar {
		t.Errorf("expected one avatar history event, got %+v", events)
	}
}

// Nothing replays history, so a superseded avatar is reclaimed rather than kept
// around forever.
func TestUploadAvatarRemovesTheSupersededFile(t *testing.T) {
	h := newHarness(t)
	h.files.url = "/uploads/first.png"
	if rec := h.upload("file", "a.png", pngBytes(64), h.token); rec.Code != http.StatusOK {
		t.Fatalf("first upload failed: %s", rec.Body)
	}
	h.files.url = "/uploads/second.png"
	if rec := h.upload("file", "b.png", pngBytes(64), h.token); rec.Code != http.StatusOK {
		t.Fatalf("second upload failed: %s", rec.Body)
	}

	found := false
	for _, r := range h.files.removed {
		if r == "/uploads/first.png" {
			found = true
		}
	}
	if !found {
		t.Errorf("the superseded file should be removed, removals = %v", h.files.removed)
	}
	// The file in use must survive.
	if !h.files.Exists("/uploads/second.png") {
		t.Error("the current avatar must not be removed")
	}
}

func TestUploadRejectsMissingFile(t *testing.T) {
	h := newHarness(t)
	rec := h.upload("wrong_field", "me.png", pngBytes(64), h.token)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestUploadRejectsOversizeFile(t *testing.T) {
	h := newHarness(t)
	// MaxUploadBytes is 2 MB in the harness.
	rec := h.upload("file", "big.png", pngBytes(3*1024*1024), h.token)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestUploadRequiresAuth(t *testing.T) {
	h := newHarness(t)
	rec := h.upload("file", "me.png", pngBytes(64), "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// ---------- history ----------

func TestHistoryDefaultsToSevenDays(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/v1/me/history", h.token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	body := decode[struct {
		Days   int                    `json:"days"`
		Events []models.ProfileChange `json:"events"`
	}](t, rec)
	if body.Days != 7 {
		t.Errorf("days = %d, want 7", body.Days)
	}
	if body.Events == nil {
		t.Error("events should be an empty array, not null")
	}
}

// ---------- admin ----------

func TestAdminRoutesRejectEmployees(t *testing.T) {
	h := newHarness(t)
	paths := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/admin/skills"},
		{http.MethodGet, "/api/v1/admin/settings"},
	}
	for _, p := range paths {
		if rec := h.do(p.method, p.path, h.token, nil); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as employee = %d, want 403", p.method, p.path, rec.Code)
		}
	}
}

func TestAdminSeesInactiveSkills(t *testing.T) {
	h := newHarness(t)
	body := decode[struct {
		Skills []models.Skill `json:"skills"`
	}](t, h.do(http.MethodGet, "/api/v1/admin/skills", h.admin, nil))
	if len(body.Skills) != 3 {
		t.Errorf("admin sees %d skills, want 3 (including the inactive one)", len(body.Skills))
	}
}

func TestAdminCreateSkill(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/v1/admin/skills", h.admin, map[string]any{
		"name": "Rust", "sort_order": 15,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	s := decode[models.Skill](t, rec)
	if s.Name != "Rust" || !s.Active {
		t.Errorf("unexpected skill: %+v", s)
	}

	// A duplicate name is a conflict.
	dup := h.do(http.MethodPost, "/api/v1/admin/skills", h.admin, map[string]any{"name": "Rust"})
	if dup.Code != http.StatusConflict {
		t.Errorf("duplicate name status = %d, want 409", dup.Code)
	}
}

func TestAdminCreateSkillRequiresName(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(http.MethodPost, "/api/v1/admin/skills", h.admin, map[string]any{
		"sort_order": 15,
	}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestAdminDeactivateSkill(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPatch, "/api/v1/admin/skills/s-go", h.admin, map[string]any{
		"active": false,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	s := decode[models.Skill](t, rec)
	if s.Active {
		t.Error("the skill should be inactive")
	}
	if s.Name != "Go" {
		t.Errorf("name should be unchanged, got %q", s.Name)
	}
}

func TestAdminSettingsRoundTrip(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPut, "/api/v1/admin/settings", h.admin, map[string]int{
		"history_retention_days": 30,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	// The new window now drives the employee's default history request.
	body := decode[struct {
		Days int `json:"days"`
	}](t, h.do(http.MethodGet, "/api/v1/me/history", h.token, nil))
	if body.Days != 30 {
		t.Errorf("history default = %d, want the configured 30", body.Days)
	}
}

func TestAdminSettingsRejectsOutOfRange(t *testing.T) {
	h := newHarness(t)
	for _, v := range []int{0, -1, 400} {
		rec := h.do(http.MethodPut, "/api/v1/admin/settings", h.admin, map[string]int{
			"history_retention_days": v,
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("value %d returned %d, want 422", v, rec.Code)
		}
	}
}

// ---------- health ----------

func TestHealth(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/healthz", "", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("unexpected body: %s", rec.Body)
	}
}

func TestRateLimiterWindow(t *testing.T) {
	rl := api.NewRateLimiter(2, time.Minute, false)
	for i := 0; i < 2; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Error("the third attempt should be blocked")
	}
	if !rl.Allow("5.6.7.8") {
		t.Error("a different IP has its own budget")
	}
}

func ExampleServer() {
	fmt.Println("see TestGetProfile for a worked request example")
	// Output: see TestGetProfile for a worked request example
}

// An employee cannot widen the window. The query parameter is gone, so a hand-made
// request asking for a year is simply ignored and the configured window applies.
func TestEmployeeCannotWidenTheHistoryWindow(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	old := "old"
	h.mem.SeedChange(models.ProfileChange{
		UserID: h.userID, Field: models.FieldMobile, OldValue: &old, NewValue: &old,
		ChangedBy: h.userID, ChangedAt: now.AddDate(0, 0, -30),
	})
	recent := "recent"
	h.mem.SeedChange(models.ProfileChange{
		UserID: h.userID, Field: models.FieldMobile, OldValue: &recent, NewValue: &recent,
		ChangedBy: h.userID, ChangedAt: now.AddDate(0, 0, -1),
	})

	type body struct {
		Days  int `json:"days"`
		Total int `json:"total"`
	}

	// The default 7-day window hides the older event.
	got := decode[body](t, h.do(http.MethodGet, "/api/v1/me/history", h.token, nil))
	if got.Days != 7 || got.Total != 1 {
		t.Fatalf("default window: %+v, want 7 days and 1 event", got)
	}

	// Asking for a year changes nothing.
	got = decode[body](t, h.do(http.MethodGet, "/api/v1/me/history?days=365", h.token, nil))
	if got.Days != 7 || got.Total != 1 {
		t.Errorf("?days=365 was honoured: %+v, want it ignored", got)
	}
}

// Only the administrator's setting moves the window, for everyone at once.
func TestAdminSettingDrivesTheHistoryWindow(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	old := "old"
	h.mem.SeedChange(models.ProfileChange{
		UserID: h.userID, Field: models.FieldMobile, OldValue: &old, NewValue: &old,
		ChangedBy: h.userID, ChangedAt: now.AddDate(0, 0, -30),
	})

	type body struct {
		Days  int `json:"days"`
		Total int `json:"total"`
	}

	got := decode[body](t, h.do(http.MethodGet, "/api/v1/me/history", h.token, nil))
	if got.Total != 0 {
		t.Fatalf("the 30-day-old event should be outside the default window: %+v", got)
	}

	if rec := h.do(http.MethodPut, "/api/v1/admin/settings", h.admin, map[string]int{
		"history_retention_days": 90,
	}); rec.Code != http.StatusOK {
		t.Fatalf("admin settings update: %d", rec.Code)
	}

	got = decode[body](t, h.do(http.MethodGet, "/api/v1/me/history", h.token, nil))
	if got.Days != 90 || got.Total != 1 {
		t.Errorf("after the admin widened it: %+v, want 90 days and 1 event", got)
	}
}
