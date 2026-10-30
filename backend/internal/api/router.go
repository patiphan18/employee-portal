package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

// Routes builds the full route table (spec section 5).
func (s *Server) Routes(corsOrigins []string, uploadDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(Recoverer, RequestLogger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", s.handleHealth)

	// Uploaded avatars. In production nginx serves these directly; the handler exists
	// so the API is usable on its own in development.
	fileServer := http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir)))
	r.Get("/uploads/*", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fileServer.ServeHTTP(w, r)
	})

	loginLimit := NewRateLimiter(10, 5*time.Minute, s.TrustProxyHeaders)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(loginLimit.Middleware)
			r.Post("/auth/login", s.handleLogin)
		})

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(s.Issuer))

			r.Get("/auth/me", s.handleTokenInfo)

			// Self-service profile: employees only.
			r.Group(func(r chi.Router) {
				r.Use(RequireEmployee)
				r.Get("/me", s.handleGetProfile)
				r.Patch("/me/contact", s.handlePatchContact)
				r.Post("/me/avatar", s.handleUploadAvatar)
				r.Put("/me/skills", s.handlePutSkills)
				r.Get("/me/history", s.handleHistory)

				// The employee-facing catalog exists to populate the skill picker.
				r.Get("/skills", s.handleListSkills)
			})

			r.Group(func(r chi.Router) {
				r.Use(RequireAdmin)
				r.Get("/admin/skills", s.handleAdminListSkills)
				r.Post("/admin/skills", s.handleAdminCreateSkill)
				r.Patch("/admin/skills/{id}", s.handleAdminUpdateSkill)
				r.Get("/admin/settings", s.handleGetSettings)
				r.Put("/admin/settings", s.handlePutSettings)
			})
		})
	})

	return r
}
