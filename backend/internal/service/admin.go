package service

import (
	"context"
	"strconv"

	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/store"
	"github.com/digithun/employee-portal/backend/internal/validate"
)

// Admin covers the configurable parts of the system: the skill catalog (FR-4.2) and
// system settings (FR-5.4).
type Admin struct {
	store       store.Store
	defaultDays int
}

func NewAdmin(s store.Store, defaultDays int) *Admin {
	return &Admin{store: s, defaultDays: defaultDays}
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
