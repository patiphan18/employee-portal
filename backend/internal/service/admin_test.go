package service_test

import (
	"context"
	"testing"

	"github.com/digithun/employee-portal/backend/internal/service"
	"github.com/digithun/employee-portal/backend/internal/store"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestCatalogFiltersInactive(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)
	ctx := context.Background()

	all, err := admin.Catalog(ctx, false)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	active, err := admin.Catalog(ctx, true)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(all) != 3 || len(active) != 2 {
		t.Errorf("all = %d, active = %d; want 3 and 2", len(all), len(active))
	}
}

func TestCreateSkillDefaults(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)

	s, err := admin.CreateSkill(context.Background(), service.SkillInput{Name: ptr("  Rust  ")})
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	if s.Name != "Rust" {
		t.Errorf("name = %q, want the trimmed Rust", s.Name)
	}
	if !s.Active || s.SortOrder != 100 {
		t.Errorf("unexpected defaults: %+v", s)
	}
	if s.ID == "" {
		t.Error("an id should be assigned")
	}
}

func TestCreateSkillValidation(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)
	ctx := context.Background()

	if _, err := admin.CreateSkill(ctx, service.SkillInput{}); err == nil {
		t.Error("a missing name should be rejected")
	}
	if _, err := admin.CreateSkill(ctx, service.SkillInput{Name: ptr("   ")}); err == nil {
		t.Error("a blank name should be rejected")
	}
	long := make([]byte, 61)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := admin.CreateSkill(ctx, service.SkillInput{Name: ptr(string(long))}); err == nil {
		t.Error("a 61-character name should be rejected")
	}
}

func TestCreateSkillRejectsDuplicateName(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)

	if _, err := admin.CreateSkill(context.Background(), service.SkillInput{Name: ptr("go")}); err != store.ErrConflict {
		t.Errorf("expected ErrConflict for a name that differs only in case, got %v", err)
	}
}

func TestUpdateSkillPatchesOnlySuppliedFields(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)
	ctx := context.Background()

	got, err := admin.UpdateSkill(ctx, "s-go", service.SkillInput{Active: boolPtr(false)})
	if err != nil {
		t.Fatalf("UpdateSkill: %v", err)
	}
	if got.Active {
		t.Error("the skill should be deactivated")
	}
	if got.Name != "Go" || got.SortOrder != 10 {
		t.Errorf("untouched fields changed: %+v", got)
	}

	got, err = admin.UpdateSkill(ctx, "s-go", service.SkillInput{
		Name: ptr("Golang"), SortOrder: intPtr(5),
	})
	if err != nil {
		t.Fatalf("UpdateSkill: %v", err)
	}
	if got.Name != "Golang" || got.SortOrder != 5 {
		t.Errorf("update did not apply: %+v", got)
	}
}

func TestUpdateSkillUnknownID(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)
	if _, err := admin.UpdateSkill(context.Background(), "nope", service.SkillInput{Name: ptr("X")}); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateSkillRejectsBlankName(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)
	if _, err := admin.UpdateSkill(context.Background(), "s-go", service.SkillInput{Name: ptr("")}); err == nil {
		t.Error("a blank name should be rejected")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)
	ctx := context.Background()

	if got := admin.Settings(ctx).HistoryRetentionDays; got != 7 {
		t.Errorf("default = %d, want 7", got)
	}

	out, err := admin.UpdateSettings(ctx, service.Settings{HistoryRetentionDays: 30})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if out.HistoryRetentionDays != 30 {
		t.Errorf("returned %d, want 30", out.HistoryRetentionDays)
	}
	if got := admin.Settings(ctx).HistoryRetentionDays; got != 30 {
		t.Errorf("reread = %d, want 30", got)
	}
}

func TestUpdateSettingsRejectsOutOfRange(t *testing.T) {
	mem, _, _ := fixture(t)
	admin := service.NewAdmin(mem, 7)
	for _, v := range []int{0, -1, 366, 10000} {
		if _, err := admin.UpdateSettings(context.Background(), service.Settings{HistoryRetentionDays: v}); err == nil {
			t.Errorf("%d should be rejected", v)
		}
	}
}

func TestSettingsClampsACorruptStoredValue(t *testing.T) {
	mem, _, _ := fixture(t)
	ctx := context.Background()
	// A value written outside the API, e.g. straight into the database.
	mem.SetSetting(ctx, store.SettingHistoryRetentionDays, "99999")

	admin := service.NewAdmin(mem, 7)
	if got := admin.Settings(ctx).HistoryRetentionDays; got != service.MaxRetentionDays {
		t.Errorf("stored 99999 surfaced as %d, want it clamped to %d", got, service.MaxRetentionDays)
	}
}
