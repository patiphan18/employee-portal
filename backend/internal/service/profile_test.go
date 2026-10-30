package service_test

import (
	"context"
	"testing"

	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/service"
	"github.com/digithun/employee-portal/backend/internal/store"
)

func ptr(s string) *string { return &s }

// fixture builds a memory store with one employee and a three-entry catalog.
func fixture(t *testing.T) (*store.Memory, *service.Profile, string) {
	t.Helper()
	mem := store.NewMemory()
	mem.SeedUser(&models.User{
		ID:        "user-1",
		Email:     "somchai@example.com",
		FirstName: "Somchai",
		LastName:  "Jaidee",
		Mobile:    ptr("0812345678"),
		Role:      models.RoleEmployee,
	})
	mem.SeedSkill(&models.Skill{ID: "s-go", Name: "Go", Active: true, SortOrder: 10})
	mem.SeedSkill(&models.Skill{ID: "s-react", Name: "React", Active: true, SortOrder: 20})
	mem.SeedSkill(&models.Skill{ID: "s-old", Name: "Flash", Active: false, SortOrder: 30})
	return mem, service.NewProfile(mem), "user-1"
}

func TestUpdateContactPatchesOnlySuppliedFields(t *testing.T) {
	mem, svc, uid := fixture(t)
	ctx := context.Background()

	if _, err := svc.UpdateContact(ctx, uid, models.ContactPatch{Address: ptr("99 Sukhumvit")}); err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}

	got, err := svc.Get(ctx, uid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Address == nil || *got.Address != "99 Sukhumvit" {
		t.Errorf("address = %v, want 99 Sukhumvit", got.Address)
	}
	if got.Mobile == nil || *got.Mobile != "0812345678" {
		t.Errorf("mobile should be untouched, got %v", got.Mobile)
	}
	if got.Email != "somchai@example.com" {
		t.Errorf("email should be untouched, got %q", got.Email)
	}

	events, _ := mem.History(ctx, uid, timeZero(), 0, 0)
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 history event, got %d", len(events))
	}
	if events[0].Field != models.FieldAddress {
		t.Errorf("event field = %q, want address", events[0].Field)
	}
}

func TestUpdateContactNeverChangesNames(t *testing.T) {
	_, svc, uid := fixture(t)
	ctx := context.Background()

	// Names are not even part of ContactPatch; this checks the stored record is intact
	// after a contact write (FR-3.1).
	if _, err := svc.UpdateContact(ctx, uid, models.ContactPatch{Mobile: ptr("0899999999")}); err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}
	got, _ := svc.Get(ctx, uid)
	if got.FirstName != "Somchai" || got.LastName != "Jaidee" {
		t.Errorf("names changed: %q %q", got.FirstName, got.LastName)
	}
}

func TestUpdateContactUnchangedValueWritesNoHistory(t *testing.T) {
	mem, svc, uid := fixture(t)
	ctx := context.Background()

	// Same number, differently formatted: normalisation makes this a no-op (FR-3.9).
	if _, err := svc.UpdateContact(ctx, uid, models.ContactPatch{Mobile: ptr("081-234-5678")}); err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}
	events, _ := mem.History(ctx, uid, timeZero(), 0, 0)
	if len(events) != 0 {
		t.Errorf("a no-op write produced %d history events, want 0", len(events))
	}
}

func TestUpdateContactValidationErrors(t *testing.T) {
	_, svc, uid := fixture(t)
	ctx := context.Background()

	_, err := svc.UpdateContact(ctx, uid, models.ContactPatch{
		Mobile: ptr("nope"),
		Email:  ptr("also-nope"),
	})
	fe, ok := service.AsFieldErrors(err)
	if !ok {
		t.Fatalf("expected FieldErrors, got %v", err)
	}
	if fe.Errors["mobile"] == "" || fe.Errors["email"] == "" {
		t.Errorf("both fields should report an error, got %v", fe.Errors)
	}
}

func TestUpdateContactRejectsDuplicateEmail(t *testing.T) {
	mem, svc, uid := fixture(t)
	mem.SeedUser(&models.User{ID: "user-2", Email: "taken@example.com", FirstName: "A", LastName: "B"})

	_, err := svc.UpdateContact(context.Background(), uid, models.ContactPatch{Email: ptr("taken@example.com")})
	if err != store.ErrEmailTaken {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestUpdateContactClearsFieldWithEmptyString(t *testing.T) {
	_, svc, uid := fixture(t)
	ctx := context.Background()

	got, err := svc.UpdateContact(ctx, uid, models.ContactPatch{Mobile: ptr("")})
	if err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}
	if got.Mobile != nil {
		t.Errorf("mobile should be cleared, got %v", *got.Mobile)
	}
}

func TestSetSkillsRecordsOneReadableEvent(t *testing.T) {
	mem, svc, uid := fixture(t)
	ctx := context.Background()

	if _, err := svc.SetSkills(ctx, uid, []string{"s-react", "s-go"}, nil); err != nil {
		t.Fatalf("SetSkills: %v", err)
	}

	events, _ := mem.History(ctx, uid, timeZero(), 0, 0)
	if len(events) != 1 {
		t.Fatalf("expected 1 skills event, got %d", len(events))
	}
	if events[0].Field != models.FieldSkills {
		t.Fatalf("field = %q, want skills", events[0].Field)
	}
	if *events[0].OldValue != "" {
		t.Errorf("old value = %q, want empty", *events[0].OldValue)
	}
	// Names are sorted so the diff reads the same regardless of selection order.
	if *events[0].NewValue != "Go, React" {
		t.Errorf("new value = %q, want %q", *events[0].NewValue, "Go, React")
	}
}

func TestSetSkillsReorderingIsNotAChange(t *testing.T) {
	mem, svc, uid := fixture(t)
	ctx := context.Background()

	if _, err := svc.SetSkills(ctx, uid, []string{"s-go", "s-react"}, nil); err != nil {
		t.Fatalf("SetSkills: %v", err)
	}
	if _, err := svc.SetSkills(ctx, uid, []string{"s-react", "s-go"}, nil); err != nil {
		t.Fatalf("SetSkills (reorder): %v", err)
	}

	events, _ := mem.History(ctx, uid, timeZero(), 0, 0)
	if len(events) != 1 {
		t.Errorf("reordering the same set produced %d events, want 1", len(events))
	}
}

func TestSetSkillsRejectsUnknownAndInactive(t *testing.T) {
	_, svc, uid := fixture(t)
	ctx := context.Background()

	if _, err := svc.SetSkills(ctx, uid, []string{"does-not-exist"}, nil); err == nil {
		t.Error("an unknown skill id should be rejected")
	}
	_, err := svc.SetSkills(ctx, uid, []string{"s-old"}, nil)
	fe, ok := service.AsFieldErrors(err)
	if !ok {
		t.Fatalf("expected FieldErrors for an inactive skill, got %v", err)
	}
	if fe.Errors["skills"] == "" {
		t.Errorf("expected a skills error, got %v", fe.Errors)
	}
}

func TestDeactivatedSkillStaysOnTheProfile(t *testing.T) {
	mem, svc, uid := fixture(t)
	ctx := context.Background()

	// The employee selected Go while it was active...
	if _, err := svc.SetSkills(ctx, uid, []string{"s-go"}, nil); err != nil {
		t.Fatalf("SetSkills: %v", err)
	}
	// ...and an admin later retires it (FR-4.3).
	mem.SeedSkill(&models.Skill{ID: "s-go", Name: "Go", Active: false, SortOrder: 10})

	got, err := svc.Get(ctx, uid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Skills) != 1 || got.Skills[0].Name != "Go" {
		t.Errorf("a deactivated skill must stay on the profile, got %+v", got.Skills)
	}
}

func TestCompleteness(t *testing.T) {
	cases := []struct {
		name    string
		user    models.User
		skills  []models.Skill
		percent int
		missing int
	}{
		{
			name:    "email only",
			user:    models.User{Email: "a@example.com"},
			percent: 20, missing: 4,
		},
		{
			name: "everything filled",
			user: models.User{
				Email: "a@example.com", Mobile: ptr("0812345678"),
				Address: ptr("addr"), AvatarURL: ptr("/uploads/a.png"),
			},
			skills:  []models.Skill{{Name: "Go"}},
			percent: 100, missing: 0,
		},
		{
			name: "whitespace does not count as filled",
			user: models.User{
				Email: "a@example.com", Mobile: ptr("   "), Address: ptr(""),
			},
			percent: 20, missing: 4,
		},
		{
			name: "no skills selected",
			user: models.User{
				Email: "a@example.com", Mobile: ptr("0812345678"),
				Address: ptr("addr"), AvatarURL: ptr("/uploads/a.png"),
			},
			percent: 80, missing: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := service.Completeness(&tc.user, tc.skills, nil)
			if got.Percent != tc.percent {
				t.Errorf("percent = %d, want %d", got.Percent, tc.percent)
			}
			if len(got.Missing) != tc.missing {
				t.Errorf("missing = %v, want %d entries", got.Missing, tc.missing)
			}
		})
	}
}

func TestDiffIgnoresUnchangedFields(t *testing.T) {
	before := &models.User{ID: "u", Email: "a@example.com", Mobile: ptr("0812345678")}
	after := &models.User{ID: "u", Email: "a@example.com", Mobile: ptr("0812345678")}
	if changes := service.Diff(before, after, "u"); len(changes) != 0 {
		t.Errorf("identical users produced %d changes", len(changes))
	}

	after = &models.User{ID: "u", Email: "b@example.com", Mobile: ptr("0899999999"), Address: ptr("x")}
	changes := service.Diff(before, after, "u")
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d: %+v", len(changes), changes)
	}
	for _, c := range changes {
		if c.ChangedBy != "u" || c.UserID != "u" {
			t.Errorf("change is not attributed to the actor: %+v", c)
		}
	}
}

func TestDiffTreatsEmailCaseInsensitively(t *testing.T) {
	before := &models.User{ID: "u", Email: "a@example.com"}
	after := &models.User{ID: "u", Email: "A@Example.com"}
	if changes := service.Diff(before, after, "u"); len(changes) != 0 {
		t.Errorf("a case-only email difference should not be a change, got %+v", changes)
	}
}

func TestSkillNamesMergesCustomEntries(t *testing.T) {
	catalog := []models.Skill{{ID: "1", Name: "Go"}, {ID: "2", Name: "React"}}
	// Sorting is case-insensitive and spans both kinds of skill.
	got := service.SkillNames(catalog, []string{"elixir", "Zig"})
	if got != "elixir, Go, React, Zig" {
		t.Errorf("SkillNames = %q", got)
	}
	if service.SkillNames(nil, nil) != "" {
		t.Error("an empty selection should render as an empty string")
	}
}
