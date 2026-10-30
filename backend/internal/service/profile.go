// Package service holds the business rules: validation, change detection, history
// writing, and completeness scoring. It depends on store interfaces only.
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/store"
	"github.com/digithun/employee-portal/backend/internal/validate"
)

// FieldErrors is returned when input fails validation (FR-3.8).
type FieldErrors struct{ Errors validate.Errors }

func (e *FieldErrors) Error() string { return "validation failed" }

var ErrEmailTaken = store.ErrEmailTaken

type Profile struct{ store store.Store }

func NewProfile(s store.Store) *Profile { return &Profile{store: s} }

// Get assembles the API profile view: the user row, their skills, and the completeness
// score.
func (p *Profile) Get(ctx context.Context, userID string) (*models.Profile, error) {
	u, err := p.store.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	skills, custom, err := p.skillsOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	return buildProfile(u, skills, custom), nil
}

func (p *Profile) skillsOf(ctx context.Context, userID string) ([]models.Skill, []string, error) {
	skills, err := p.store.UserSkills(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	custom, err := p.store.UserCustomSkills(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return skills, custom, nil
}

// MergeSkills presents catalog picks and the employee's own entries as one list,
// sorted by name so the profile reads consistently however it was assembled.
func MergeSkills(catalog []models.Skill, custom []string) []models.ProfileSkill {
	out := make([]models.ProfileSkill, 0, len(catalog)+len(custom))
	for _, s := range catalog {
		out = append(out, models.ProfileSkill{ID: s.ID, Name: s.Name})
	}
	for _, name := range custom {
		out = append(out, models.ProfileSkill{Name: name, Custom: true})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func buildProfile(u *models.User, skills []models.Skill, custom []string) *models.Profile {
	return &models.Profile{
		ID:           u.ID,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Email:        u.Email,
		Mobile:       u.Mobile,
		Address:      u.Address,
		AvatarURL:    u.AvatarURL,
		Role:         u.Role,
		Skills:       MergeSkills(skills, custom),
		Completeness: Completeness(u, skills, custom),
		UpdatedAt:    u.UpdatedAt,
	}
}

// Completeness scores the profile out of 100 (FR-7.2).
func Completeness(u *models.User, skills []models.Skill, custom []string) models.Completeness {
	type item struct {
		label  string
		filled bool
	}
	items := []item{
		{"avatar", nonEmpty(u.AvatarURL)},
		{"mobile", nonEmpty(u.Mobile)},
		{"email", strings.TrimSpace(u.Email) != ""},
		{"address", nonEmpty(u.Address)},
		{"skills", len(skills)+len(custom) > 0},
	}
	missing := make([]string, 0, len(items))
	filled := 0
	for _, it := range items {
		if it.filled {
			filled++
		} else {
			missing = append(missing, it.label)
		}
	}
	return models.Completeness{
		Percent: filled * 100 / len(items),
		Missing: missing,
	}
}

func nonEmpty(s *string) bool { return s != nil && strings.TrimSpace(*s) != "" }

// UpdateContact applies a partial contact patch. Absent keys are untouched (FR-3.3) and
// unchanged values write no history (FR-3.9).
func (p *Profile) UpdateContact(ctx context.Context, userID string, patch models.ContactPatch) (*models.Profile, error) {
	u, err := p.store.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	errs := validate.Errors{}
	updated := *u

	if patch.Mobile != nil {
		v, msg := validate.Mobile(*patch.Mobile)
		if msg != "" {
			errs.Add("mobile", msg)
		} else {
			updated.Mobile = optional(v)
		}
	}
	if patch.Email != nil {
		v, msg := validate.Email(*patch.Email)
		if msg != "" {
			errs.Add("email", msg)
		} else {
			updated.Email = v
		}
	}
	if patch.Address != nil {
		v, msg := validate.Address(*patch.Address)
		if msg != "" {
			errs.Add("address", msg)
		} else {
			updated.Address = optional(v)
		}
	}
	if errs.Any() {
		return nil, &FieldErrors{Errors: errs}
	}

	changes := Diff(u, &updated, userID)
	if len(changes) == 0 {
		// Nothing moved: skip the write entirely but still answer with the profile.
		skills, custom, err := p.skillsOf(ctx, userID)
		if err != nil {
			return nil, err
		}
		return buildProfile(u, skills, custom), nil
	}

	if err := p.store.UpdateUser(ctx, &updated, changes); err != nil {
		return nil, err
	}
	return p.Get(ctx, userID)
}

// SetAvatar records a newly uploaded avatar URL and its history row.
func (p *Profile) SetAvatar(ctx context.Context, userID, url string) (*models.Profile, error) {
	u, err := p.store.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	updated := *u
	updated.AvatarURL = optional(url)
	changes := Diff(u, &updated, userID)
	if len(changes) > 0 {
		if err := p.store.UpdateUser(ctx, &updated, changes); err != nil {
			return nil, err
		}
	}
	return p.Get(ctx, userID)
}

// Diff produces one change per mutated auditable field. Names are never compared:
// they cannot change through this API (FR-3.1).
func Diff(before, after *models.User, actorID string) []models.ProfileChange {
	var changes []models.ProfileChange
	add := func(field string, old, new *string) {
		if deref(old) == deref(new) {
			return
		}
		changes = append(changes, models.ProfileChange{
			UserID:    after.ID,
			Field:     field,
			OldValue:  old,
			NewValue:  new,
			ChangedBy: actorID,
		})
	}
	add(models.FieldMobile, before.Mobile, after.Mobile)
	add(models.FieldAddress, before.Address, after.Address)
	add(models.FieldAvatar, before.AvatarURL, after.AvatarURL)
	if !strings.EqualFold(before.Email, after.Email) {
		b, a := before.Email, after.Email
		changes = append(changes, models.ProfileChange{
			UserID:    after.ID,
			Field:     models.FieldEmail,
			OldValue:  &b,
			NewValue:  &a,
			ChangedBy: actorID,
		})
	}
	return changes
}

// SetSkills replaces the selection, rejecting unknown or inactive ids (FR-3.7), and
// records a single readable change event (FR-5.7). customSkills are free-text skills
// the employee typed because the catalog did not offer them (FR-3.10).
func (p *Profile) SetSkills(ctx context.Context, userID string, skillIDs, customSkills []string) (*models.Profile, error) {
	ids, msg := validate.SkillSelection(skillIDs)
	if msg != "" {
		return nil, &FieldErrors{Errors: validate.Errors{"skills": msg}}
	}
	typed, msg := validate.CustomSkills(customSkills)
	if msg != "" {
		return nil, &FieldErrors{Errors: validate.Errors{"custom_skills": msg}}
	}

	catalog, err := p.store.ListSkills(ctx, false)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]models.Skill, len(catalog))
	byName := make(map[string]models.Skill, len(catalog))
	for _, s := range catalog {
		byID[s.ID] = s
		byName[strings.ToLower(s.Name)] = s
	}

	current, err := p.store.UserSkills(ctx, userID)
	if err != nil {
		return nil, err
	}
	currentCustom, err := p.store.UserCustomSkills(ctx, userID)
	if err != nil {
		return nil, err
	}
	// A deactivated skill the employee already holds stays selectable, because
	// FR-4.3 promises deactivation does not strip existing selections. Only newly
	// added inactive skills are rejected, so retiring a skill never blocks an
	// employee from making an unrelated edit.
	held := make(map[string]bool, len(current))
	for _, s := range current {
		held[s.ID] = true
	}

	selected := make([]models.Skill, 0, len(ids))
	for _, id := range ids {
		s, ok := byID[id]
		if !ok {
			return nil, &FieldErrors{Errors: validate.Errors{
				"skills": fmt.Sprintf("unknown skill %q", id)}}
		}
		if !s.Active && !held[id] {
			return nil, &FieldErrors{Errors: validate.Errors{
				"skills": fmt.Sprintf("skill %q is no longer available", s.Name)}}
		}
		selected = append(selected, s)
	}

	// A typed skill that turns out to exist in the catalog is folded into the catalog
	// selection, so the employee never ends up holding "React" twice — once as a pick
	// and once as their own text.
	chosen := make(map[string]bool, len(ids))
	for _, id := range ids {
		chosen[id] = true
	}
	kept := make([]string, 0, len(typed))
	for _, name := range typed {
		if s, ok := byName[strings.ToLower(name)]; ok && s.Active {
			if !chosen[s.ID] {
				chosen[s.ID] = true
				ids = append(ids, s.ID)
				selected = append(selected, s)
			}
			continue
		}
		kept = append(kept, name)
	}

	if len(selected)+len(kept) > validate.MaxSkills {
		return nil, &FieldErrors{Errors: validate.Errors{
			"skills": "at most 50 skills can be selected"}}
	}

	oldNames := SkillNames(current, currentCustom)
	newNames := SkillNames(selected, kept)
	if oldNames == newNames {
		// Same set, possibly reordered: no change event (FR-5.7).
		u, err := p.store.UserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		return buildProfile(u, current, currentCustom), nil
	}

	changes := []models.ProfileChange{{
		UserID:    userID,
		Field:     models.FieldSkills,
		OldValue:  &oldNames,
		NewValue:  &newNames,
		ChangedBy: userID,
	}}
	if err := p.store.SetUserSkills(ctx, userID, ids, kept, changes); err != nil {
		return nil, err
	}
	return p.Get(ctx, userID)
}

// SkillNames renders a selection as a sorted, comma-joined string for the audit log.
// Catalog picks and the employee's own entries read the same way, because the log is
// about what the profile said, not where each name came from.
func SkillNames(skills []models.Skill, custom []string) string {
	names := make([]string, 0, len(skills)+len(custom))
	for _, s := range skills {
		names = append(names, s.Name)
	}
	names = append(names, custom...)
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return strings.Join(names, ", ")
}

func optional(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// AsFieldErrors unwraps a validation error, if that is what err is.
func AsFieldErrors(err error) (*FieldErrors, bool) {
	var fe *FieldErrors
	ok := errors.As(err, &fe)
	return fe, ok
}
