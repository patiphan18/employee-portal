package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/digithun/employee-portal/backend/internal/validate"
)

// ErrorBody is the single error envelope used by every endpoint.
type ErrorBody struct {
	Error  string          `json:"error"`
	Errors validate.Errors `json:"errors,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorBody{Error: msg})
}

func writeFieldErrors(w http.ResponseWriter, errs validate.Errors) {
	writeJSON(w, http.StatusUnprocessableEntity, ErrorBody{
		Error:  "validation failed",
		Errors: errs,
	})
}

// decodeJSON reads a JSON body under a 1 MB cap. Unknown keys are ignored rather than
// rejected: that is what makes a contact patch carrying first_name a silent no-op for
// the name instead of an error (FR-3.1).
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(dst)
}
