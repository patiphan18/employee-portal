package service_test

import (
	"context"
	"testing"

	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/service"
)

// Regression for the review finding "retired skill blocks every later skill save".
// An admin deactivating a skill must not stop an employee who holds it from making
// an unrelated change (FR-4.3).
func TestRetiredSkillDoesNotBlockUnrelatedSave(t *testing.T) {
	mem, svc, uid := fixture(t)
	ctx := context.Background()

	if _, err := svc.SetSkills(ctx, uid, []string{"s-go"}, nil); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// An admin retires Go after the employee selected it.
	mem.SeedSkill(&models.Skill{ID: "s-go", Name: "Go", Active: false, SortOrder: 10})

	// The employee adds React; the UI submits the full set it rendered, Go included.
	p, err := svc.SetSkills(ctx, uid, []string{"s-go", "s-react"}, nil)
	if err != nil {
		t.Fatalf("adding a skill must not fail because of a retired one: %v", err)
	}
	if len(p.Skills) != 2 {
		t.Errorf("expected both skills to be kept, got %+v", p.Skills)
	}
}

// Newly adding a retired skill is still refused: the exemption covers only skills
// the employee already holds.
func TestRetiredSkillCannotBeNewlyAdded(t *testing.T) {
	_, svc, uid := fixture(t)

	_, err := svc.SetSkills(context.Background(), uid, []string{"s-old"}, nil)
	if _, ok := service.AsFieldErrors(err); !ok {
		t.Errorf("adding a never-held retired skill should still be rejected, got %v", err)
	}
}

// --- FR-3.10: an employee may add a skill the catalog does not offer ---

func TestEmployeeAddsOwnFreeTextSkill(t *testing.T) {
	mem, svc, uid := fixture(t)
	ctx := context.Background()

	p, err := svc.SetSkills(ctx, uid, []string{"s-go"}, []string{"Elixir", "Zig"})
	if err != nil {
		t.Fatalf("SetSkills: %v", err)
	}
	if len(p.Skills) != 3 {
		t.Fatalf("expected 3 skills, got %+v", p.Skills)
	}

	byName := map[string]bool{}
	for _, s := range p.Skills {
		byName[s.Name] = s.Custom
	}
	if byName["Go"] {
		t.Error("Go came from the catalog and must not be marked custom")
	}
	if !byName["Elixir"] || !byName["Zig"] {
		t.Errorf("typed skills should be marked custom, got %+v", p.Skills)
	}

	// A custom skill never leaks into the shared catalog.
	catalog, _ := mem.ListSkills(ctx, false)
	for _, s := range catalog {
		if s.Name == "Elixir" {
			t.Error("a personal skill must not be added to the shared catalog")
		}
	}
}

func TestCustomSkillsSurviveAReload(t *testing.T) {
	_, svc, uid := fixture(t)
	ctx := context.Background()

	if _, err := svc.SetSkills(ctx, uid, nil, []string{"Elixir"}); err != nil {
		t.Fatalf("SetSkills: %v", err)
	}
	p, err := svc.Get(ctx, uid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(p.Skills) != 1 || p.Skills[0].Name != "Elixir" || !p.Skills[0].Custom {
		t.Errorf("custom skill not persisted, got %+v", p.Skills)
	}
}

// Typing the name of a skill that already exists folds into the catalog pick, so the
// employee never holds the same skill twice.
func TestTypingAnExistingSkillNameFoldsIntoTheCatalogPick(t *testing.T) {
	_, svc, uid := fixture(t)

	p, err := svc.SetSkills(context.Background(), uid, nil, []string{"go"})
	if err != nil {
		t.Fatalf("SetSkills: %v", err)
	}
	if len(p.Skills) != 1 {
		t.Fatalf("expected a single skill, got %+v", p.Skills)
	}
	if p.Skills[0].Custom || p.Skills[0].ID != "s-go" {
		t.Errorf("typed name should resolve to the catalog entry, got %+v", p.Skills[0])
	}
	if p.Skills[0].Name != "Go" {
		t.Errorf("the catalog spelling should win, got %q", p.Skills[0].Name)
	}
}

func TestCustomSkillsAreDedupedCaseInsensitively(t *testing.T) {
	_, svc, uid := fixture(t)

	p, err := svc.SetSkills(context.Background(), uid, nil, []string{"Elixir", "elixir", "  ELIXIR  "})
	if err != nil {
		t.Fatalf("SetSkills: %v", err)
	}
	if len(p.Skills) != 1 {
		t.Errorf("expected one skill after deduping, got %+v", p.Skills)
	}
}

func TestCustomSkillCountsTowardsCompleteness(t *testing.T) {
	_, svc, uid := fixture(t)
	ctx := context.Background()

	before, _ := svc.Get(ctx, uid)
	p, err := svc.SetSkills(ctx, uid, nil, []string{"Elixir"})
	if err != nil {
		t.Fatalf("SetSkills: %v", err)
	}
	if p.Completeness.Percent <= before.Completeness.Percent {
		t.Errorf("a custom skill should count: %d -> %d",
			before.Completeness.Percent, p.Completeness.Percent)
	}
}

// A comma would be split back into two skills when the change is undone, so it is
// refused at the edge.
func TestCustomSkillRejectsACommaInTheName(t *testing.T) {
	_, svc, uid := fixture(t)

	_, err := svc.SetSkills(context.Background(), uid, nil, []string{"React, Native"})
	fe, ok := service.AsFieldErrors(err)
	if !ok {
		t.Fatalf("expected a field error, got %v", err)
	}
	if fe.Errors["custom_skills"] == "" {
		t.Errorf("expected a custom_skills error, got %v", fe.Errors)
	}
}

func TestSkillLimitCountsBothKinds(t *testing.T) {
	_, svc, uid := fixture(t)

	many := make([]string, 51)
	for i := range many {
		many[i] = "Skill" + string(rune('A'+i%26)) + string(rune('0'+i/26))
	}
	if _, err := svc.SetSkills(context.Background(), uid, []string{"s-go"}, many); err == nil {
		t.Error("exceeding the 50-skill limit should be rejected")
	}
}
