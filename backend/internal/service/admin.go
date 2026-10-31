package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/digithun/employee-portal/backend/internal/auth"
	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/store"
	"github.com/digithun/employee-portal/backend/internal/validate"
)

var (
	ErrSelfDelete     = errors.New("administrators cannot delete their own account")
	ErrSelfRoleChange = errors.New("administrators cannot change their own role")
)

// Admin covers account management and configurable system features.
type Admin struct {
	store       store.Store
	defaultDays int
}

func NewAdmin(s store.Store, defaultDays int) *Admin {
	return &Admin{store: s, defaultDays: defaultDays}
}

const AdminUserPageSize = 10

type UserPage struct {
	Users      []models.User `json:"users"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	Total      int           `json:"total"`
	TotalPages int           `json:"total_pages"`
}

func (a *Admin) Users(ctx context.Context, page int) (*UserPage, error) {
	if page < 1 {
		page = 1
	}
	// Bound offset arithmetic even for malformed, extremely large query values.
	if page > 100_000_000 {
		page = 100_000_000
	}
	users, total, err := a.store.ListUsers(ctx, AdminUserPageSize, (page-1)*AdminUserPageSize)
	if err != nil {
		return nil, err
	}
	totalPages := (total + AdminUserPageSize - 1) / AdminUserPageSize
	if totalPages < 1 {
		totalPages = 1
	}
	return &UserPage{Users: users, Page: page, PerPage: AdminUserPageSize, Total: total, TotalPages: totalPages}, nil
}

type ManagedUserInput struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Mobile    string `json:"mobile"`
	Address   string `json:"address"`
	Role      string `json:"role"`
	Password  string `json:"password"`
}

func (a *Admin) CreateUser(ctx context.Context, in ManagedUserInput) (*models.User, error) {
	u, errs := normalizeManagedUser(in)
	if len(errs) > 0 {
		return nil, &FieldErrors{Errors: errs}
	}
	if err := validateManagedPassword(in.Password, true); err != "" {
		return nil, &FieldErrors{Errors: validate.Errors{"password": err}}
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u.PasswordHash = hash
	if err := a.store.CreateManagedUser(ctx, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *Admin) UpdateUser(ctx context.Context, id, actorID string, in ManagedUserInput) (*models.User, error) {
	current, err := a.store.UserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u, errs := normalizeManagedUser(in)
	if len(errs) > 0 {
		return nil, &FieldErrors{Errors: errs}
	}
	if id == actorID && u.Role != current.Role {
		return nil, ErrSelfRoleChange
	}
	u.ID = id
	u.AvatarURL = current.AvatarURL
	u.CreatedAt = current.CreatedAt
	if in.Password != "" {
		if msg := validateManagedPassword(in.Password, false); msg != "" {
			return nil, &FieldErrors{Errors: validate.Errors{"password": msg}}
		}
		u.PasswordHash, err = auth.HashPassword(in.Password)
		if err != nil {
			return nil, err
		}
	}
	if err := a.store.UpdateManagedUser(ctx, &u); err != nil {
		return nil, err
	}
	return a.store.UserByID(ctx, id)
}

func (a *Admin) DeleteUser(ctx context.Context, id, actorID string) error {
	if id == actorID {
		return ErrSelfDelete
	}
	return a.store.DeleteUser(ctx, id)
}

func normalizeManagedUser(in ManagedUserInput) (models.User, validate.Errors) {
	errs := validate.Errors{}
	u := models.User{FirstName: strings.TrimSpace(in.FirstName), LastName: strings.TrimSpace(in.LastName), Role: in.Role}
	if u.FirstName == "" || utf8.RuneCountInString(u.FirstName) > 100 {
		errs.Add("first_name", "must be between 1 and 100 characters")
	}
	if u.LastName == "" || utf8.RuneCountInString(u.LastName) > 100 {
		errs.Add("last_name", "must be between 1 and 100 characters")
	}
	email, msg := validate.Email(in.Email)
	u.Email = email
	if msg != "" {
		errs.Add("email", msg)
	}
	if u.Role != models.RoleEmployee && u.Role != models.RoleAdmin {
		errs.Add("role", "must be employee or admin")
	}
	mobile, msg := validate.Mobile(in.Mobile)
	if msg != "" {
		errs.Add("mobile", msg)
	} else if mobile != "" {
		u.Mobile = &mobile
	}
	address, msg := validate.Address(in.Address)
	if msg != "" {
		errs.Add("address", msg)
	} else if address != "" {
		u.Address = &address
	}
	return u, errs
}

func validateManagedPassword(password string, required bool) string {
	if password == "" && !required {
		return ""
	}
	if strings.TrimSpace(password) == "" {
		return "must not be blank"
	}
	if utf8.RuneCountInString(password) < 8 {
		return "must be at least 8 characters"
	}
	if len(password) > 72 {
		return "must be no more than 72 bytes"
	}
	return ""
}

// Catalog returns the skill list. Employees see only active entries.
func (a *Admin) Catalog(ctx context.Context, activeOnly bool) ([]models.Skill, error) {
	return a.store.ListSkills(ctx, activeOnly)
}

// SkillInput is the admin create/update payload. Nil fields are left unchanged on update.
type SkillInput struct {
	Name      *string `json:"name"`
	Active    *bool   `json:"active"`
	SortOrder *int    `json:"sort_order"`
}

func (a *Admin) CreateSkill(ctx context.Context, in SkillInput) (*models.Skill, error) {
	errs := validate.Errors{}
	name := ""
	if in.Name == nil {
		errs.Add("name", "name is required")
	} else {
		v, msg := validate.SkillName(*in.Name)
		if msg != "" {
			errs.Add("name", msg)
		}
		name = v
	}
	if errs.Any() {
		return nil, &FieldErrors{Errors: errs}
	}

	s := &models.Skill{
		Name:      name,
		Active:    true,
		SortOrder: 100,
	}
	if in.Active != nil {
		s.Active = *in.Active
	}
	if in.SortOrder != nil {
		s.SortOrder = *in.SortOrder
	}
	if err := a.store.CreateSkill(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// UpdateSkill renames, reorders, or deactivates. Deactivation is soft (FR-4.3):
// nothing is deleted, so employees keep skills they already selected.
func (a *Admin) UpdateSkill(ctx context.Context, id string, in SkillInput) (*models.Skill, error) {
	s, err := a.store.SkillByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		v, msg := validate.SkillName(*in.Name)
		if msg != "" {
			return nil, &FieldErrors{Errors: validate.Errors{"name": msg}}
		}
		s.Name = v
	}
	if in.Active != nil {
		s.Active = *in.Active
	}
	if in.SortOrder != nil {
		s.SortOrder = *in.SortOrder
	}
	if err := a.store.UpdateSkill(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Settings is the admin-visible configuration block.
type Settings struct {
	HistoryRetentionDays int `json:"history_retention_days"`
}

func (a *Admin) Settings(ctx context.Context) Settings {
	days := a.defaultDays
	if v, err := a.store.Setting(ctx, store.SettingHistoryRetentionDays); err == nil {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			days = n
		}
	}
	return Settings{HistoryRetentionDays: ClampDays(days)}
}

func (a *Admin) UpdateSettings(ctx context.Context, in Settings) (Settings, error) {
	if in.HistoryRetentionDays < MinRetentionDays || in.HistoryRetentionDays > MaxRetentionDays {
		return Settings{}, &FieldErrors{Errors: validate.Errors{
			"history_retention_days": "must be between 1 and 365"}}
	}
	if err := a.store.SetSetting(ctx, store.SettingHistoryRetentionDays,
		strconv.Itoa(in.HistoryRetentionDays)); err != nil {
		return Settings{}, err
	}
	return a.Settings(ctx), nil
}
