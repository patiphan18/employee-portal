// Package store defines persistence interfaces and their Postgres and in-memory
// implementations. The service layer depends only on the interface, which is what lets
// the handler tests run without a database.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/digithun/employee-portal/backend/internal/models"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already in use")
	ErrConflict   = errors.New("conflict")
	ErrLastAdmin  = errors.New("cannot remove the last administrator")
)

// Store is the full persistence surface of the application.
type Store interface {
	Ping(ctx context.Context) error

	UserByID(ctx context.Context, id string) (*models.User, error)
	UserByEmail(ctx context.Context, email string) (*models.User, error)
	ListUsers(ctx context.Context, limit, offset int) ([]models.User, int, error)
	CreateManagedUser(ctx context.Context, u *models.User) error
	UpdateManagedUser(ctx context.Context, u *models.User) error
	DeleteUser(ctx context.Context, id string) error
	// UpdateUser persists the mutable user columns and the accompanying audit rows in
	// one transaction, so a profile can never move without its history (D4).
	UpdateUser(ctx context.Context, u *models.User, changes []models.ProfileChange) error

	ListSkills(ctx context.Context, activeOnly bool) ([]models.Skill, error)
	SkillByID(ctx context.Context, id string) (*models.Skill, error)
	CreateSkill(ctx context.Context, s *models.Skill) error
	UpdateSkill(ctx context.Context, s *models.Skill) error

	UserSkills(ctx context.Context, userID string) ([]models.Skill, error)
	// UserCustomSkills returns the free-text skills that belong to this employee
	// alone (FR-3.10), sorted by name.
	UserCustomSkills(ctx context.Context, userID string) ([]string, error)
	// SetUserSkills replaces both the catalog selection and the employee's own
	// free-text skills, and writes the audit rows, in one transaction.
	SetUserSkills(ctx context.Context, userID string, skillIDs, customSkills []string, changes []models.ProfileChange) error

	// History returns one page of the user's events newer than since, newest first.
	// A limit of 0 or less means "no limit".
	History(ctx context.Context, userID string, since time.Time, limit, offset int) ([]models.ProfileChange, error)
	// HistoryCount is the total number of events in the same window, which the UI
	// needs to render page controls.
	HistoryCount(ctx context.Context, userID string, since time.Time) (int, error)

	Setting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// Setting keys.
const SettingHistoryRetentionDays = "history_retention_days"
