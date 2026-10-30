package api

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/digithun/employee-portal/backend/internal/auth"
	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/service"
	"github.com/digithun/employee-portal/backend/internal/storage"
	"github.com/digithun/employee-portal/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

// Server holds the dependencies every handler needs.
type Server struct {
	Store          store.Store
	Issuer         *auth.Issuer
	Profile        *service.Profile
	History        *service.History
	Admin          *service.Admin
	Storage        storage.Storage
	MaxUploadBytes int64
	// TrustProxyHeaders mirrors config.TrustProxyHeaders (see ClientIP).
	TrustProxyHeaders bool
}

// ---------- auth ----------

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Role      string    `json:"role"`
	Email     string    `json:"email"`
	// Profile is null for administrators, who have no employee profile (FR-2.4).
	// The client routes on Role, not on the presence of this field.
	Profile *models.Profile `json:"profile"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// One generic message for both unknown email and wrong password (FR-1.6).
	const generic = "invalid email or password"

	u, err := s.Store.UserByEmail(r.Context(), req.Email)
	if err != nil || !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, generic)
		return
	}

	token, exp, err := s.Issuer.Issue(u.ID, u.Email, u.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	resp := loginResponse{Token: token, ExpiresAt: exp, Role: u.Role, Email: u.Email}
	if u.Role != models.RoleAdmin {
		profile, err := s.Profile.Get(r.Context(), u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load profile")
			return
		}
		resp.Profile = profile
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTokenInfo(w http.ResponseWriter, r *http.Request) {
	claims, _ := ClaimsFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": claims.Subject,
		"email":   claims.Email,
		"role":    claims.Role,
	})
}

// ---------- profile ----------

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.Profile.Get(r.Context(), userID(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handlePatchContact(w http.ResponseWriter, r *http.Request) {
	var patch models.ContactPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if patch.IsEmpty() {
		writeError(w, http.StatusBadRequest, "no contact fields supplied")
		return
	}
	profile, err := s.Profile.UpdateContact(r.Context(), userID(r), patch)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	// Cap the body before parsing so an oversized upload is never buffered (FR-3.2).
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxUploadBytes+1024)
	if err := r.ParseMultipartForm(s.MaxUploadBytes); err != nil {
		writeFieldErrors(w, map[string]string{"file": "file is too large (max 2 MB)"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeFieldErrors(w, map[string]string{"file": "no file supplied"})
		return
	}
	defer file.Close()

	if header.Size > s.MaxUploadBytes {
		writeFieldErrors(w, map[string]string{"file": "file is too large (max 2 MB)"})
		return
	}

	url, err := s.Storage.Save(io.LimitReader(file, s.MaxUploadBytes))
	if err != nil {
		if errors.Is(err, storage.ErrUnsupportedType) {
			writeFieldErrors(w, map[string]string{"file": err.Error()})
			return
		}
		writeError(w, http.StatusInternalServerError, "could not store the file")
		return
	}

	previous, _ := s.Store.UserByID(r.Context(), userID(r))
	profile, err := s.Profile.SetAvatar(r.Context(), userID(r), url)
	if err != nil {
		s.Storage.Remove(url) // do not leak an orphan file if the write failed
		s.fail(w, err)
		return
	}
	// Nothing replays history any more, so the superseded file serves no purpose and
	// is reclaimed. The history row keeps the old URL for the record; it is a log of
	// what the profile said, not a promise that the bytes still exist.
	if previous != nil && previous.AvatarURL != nil && *previous.AvatarURL != url {
		s.Storage.Remove(*previous.AvatarURL) // best effort
	}
	writeJSON(w, http.StatusOK, profile)
}

type skillsRequest struct {
	SkillIDs []string `json:"skill_ids"`
	// Free-text skills the employee added because the catalog lacked them.
	CustomSkills []string `json:"custom_skills"`
}

func (s *Server) handlePutSkills(w http.ResponseWriter, r *http.Request) {
	var req skillsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	profile, err := s.Profile.SetSkills(r.Context(), userID(r), req.SkillIDs, req.CustomSkills)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// ---------- history ----------

// intParam reads an optional positive integer query parameter.
func intParam(r *http.Request, name string) (*int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, false
	}
	return &n, true
}

// The window is not a request parameter: an administrator sets it for everyone
// (FR-5.4), so there is nothing here for an employee to widen.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	page, ok := intParam(r, "page")
	if !ok {
		writeFieldErrors(w, map[string]string{"page": "must be a number"})
		return
	}
	perPage, ok := intParam(r, "per_page")
	if !ok {
		writeFieldErrors(w, map[string]string{"per_page": "must be a number"})
		return
	}

	result, err := s.History.List(r.Context(), userID(r), page, perPage)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------- skills catalog ----------

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := s.Admin.Catalog(r.Context(), true)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills})
}

func (s *Server) handleAdminListSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := s.Admin.Catalog(r.Context(), false)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills})
}

func (s *Server) handleAdminCreateSkill(w http.ResponseWriter, r *http.Request) {
	var in service.SkillInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	skill, err := s.Admin.CreateSkill(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, skill)
}

func (s *Server) handleAdminUpdateSkill(w http.ResponseWriter, r *http.Request) {
	var in service.SkillInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	skill, err := s.Admin.UpdateSkill(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

// ---------- settings ----------

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Admin.Settings(r.Context()))
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var in service.Settings
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	out, err := s.Admin.UpdateSettings(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- health ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// fail maps a domain error onto a status code, so no handler has to repeat the mapping.
func (s *Server) fail(w http.ResponseWriter, err error) {
	if fe, ok := service.AsFieldErrors(err); ok {
		writeFieldErrors(w, fe.Errors)
		return
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email already in use")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "already exists")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
