package service_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/service"
	"github.com/digithun/employee-portal/backend/internal/store"
)

func timeZero() time.Time { return time.Time{} }

func TestClampDays(t *testing.T) {
	cases := map[int]int{
		7:    7,
		1:    1,
		365:  365,
		0:    7, // nonsense falls back to the default
		-5:   7,
		400:  365,
		1000: 365,
	}
	for in, want := range cases {
		if got := service.ClampDays(in); got != want {
			t.Errorf("ClampDays(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestResolveDaysPrecedence(t *testing.T) {
	ctx := context.Background()
	mem := store.NewMemory()
	h := service.NewHistory(mem, 14)

	// No setting row: the env default wins.
	if got := h.ResolveDays(ctx); got != 14 {
		t.Errorf("env default = %d, want 14", got)
	}

	// A settings row overrides the env default.
	mem.SetSetting(ctx, store.SettingHistoryRetentionDays, "30")
	if got := h.ResolveDays(ctx); got != 30 {
		t.Errorf("settings value = %d, want 30", got)
	}

	// A value outside the range is clamped, not rejected.
	mem.SetSetting(ctx, store.SettingHistoryRetentionDays, "9999")
	if got := h.ResolveDays(ctx); got != 365 {
		t.Errorf("clamped setting = %d, want 365", got)
	}

	// A corrupt settings row falls back instead of exploding.
	mem.SetSetting(ctx, store.SettingHistoryRetentionDays, "not-a-number")
	if got := h.ResolveDays(ctx); got != 14 {
		t.Errorf("corrupt setting = %d, want the env default 14", got)
	}
}

func TestListAppliesTheWindow(t *testing.T) {
	ctx := context.Background()
	mem, _, uid := fixture(t)
	h := service.NewHistory(mem, 7)

	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	h.SetClock(func() time.Time { return now })

	mem.SeedChange(models.ProfileChange{
		UserID: uid, Field: models.FieldMobile, OldValue: ptr("a"), NewValue: ptr("b"),
		ChangedBy: uid, ChangedAt: now.AddDate(0, 0, -1),
	})
	mem.SeedChange(models.ProfileChange{
		UserID: uid, Field: models.FieldAddress, OldValue: ptr("c"), NewValue: ptr("d"),
		ChangedBy: uid, ChangedAt: now.AddDate(0, 0, -10),
	})
	// Another user's event must never appear.
	mem.SeedChange(models.ProfileChange{
		UserID: "someone-else", Field: models.FieldMobile,
		ChangedBy: "someone-else", ChangedAt: now.AddDate(0, 0, -1),
	})

	got, err := h.List(ctx, uid, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Days != 7 {
		t.Errorf("days = %d, want the 7-day default", got.Days)
	}
	if len(got.Events) != 1 || got.Total != 1 {
		t.Fatalf("7-day window returned %d of %d events, want 1", len(got.Events), got.Total)
	}
	if got.Events[0].Field != models.FieldMobile {
		t.Errorf("wrong event returned: %+v", got.Events[0])
	}

	// Only an administrator can widen the window, by changing the setting.
	mem.SetSetting(ctx, store.SettingHistoryRetentionDays, "30")
	got, err = h.List(ctx, uid, nil, nil)
	if err != nil {
		t.Fatalf("List after widening: %v", err)
	}
	if got.Days != 30 || got.Total != 2 || len(got.Events) != 2 {
		t.Errorf("30-day window returned %d of %d over %d days, want 2 over 30",
			len(got.Events), got.Total, got.Days)
	}
	// Newest first.
	if !got.Events[0].ChangedAt.After(got.Events[1].ChangedAt) {
		t.Error("events should be ordered newest first")
	}
}

func TestClampPageSize(t *testing.T) {
	cases := map[int]int{
		0:    service.DefaultPageSize,
		-5:   service.DefaultPageSize,
		1:    1,
		20:   20,
		100:  100,
		101:  service.MaxPageSize,
		5000: service.MaxPageSize,
	}
	for in, want := range cases {
		if got := service.ClampPageSize(in); got != want {
			t.Errorf("ClampPageSize(%d) = %d, want %d", in, got, want)
		}
	}
}

// seedEvents puts n events in the window, newest first by construction.
func seedEvents(mem *store.Memory, uid string, now time.Time, n int) {
	for i := 0; i < n; i++ {
		v := strconv.Itoa(i)
		mem.SeedChange(models.ProfileChange{
			UserID: uid, Field: models.FieldMobile, OldValue: &v, NewValue: &v,
			ChangedBy: uid, ChangedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}
}

func TestListPaginates(t *testing.T) {
	ctx := context.Background()
	mem, _, uid := fixture(t)
	h := service.NewHistory(mem, 7)
	now := time.Now()
	h.SetClock(func() time.Time { return now })
	seedEvents(mem, uid, now, 25)

	size := 10
	first, err := h.List(ctx, uid, nil, &size)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if first.Total != 25 {
		t.Errorf("total = %d, want 25", first.Total)
	}
	if first.TotalPages != 3 {
		t.Errorf("total_pages = %d, want 3", first.TotalPages)
	}
	if len(first.Events) != 10 {
		t.Fatalf("page 1 has %d events, want 10", len(first.Events))
	}
	if first.Page != 1 || first.PerPage != 10 {
		t.Errorf("unexpected paging: %+v", first)
	}

	// The last page holds the remainder.
	three := 3
	last, err := h.List(ctx, uid, &three, &size)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(last.Events) != 5 {
		t.Errorf("page 3 has %d events, want 5", len(last.Events))
	}

	// Pages must not overlap: the newest event appears only on page 1.
	two := 2
	second, _ := h.List(ctx, uid, &two, &size)
	seen := map[string]bool{}
	for _, e := range append(append([]models.ProfileChange{}, first.Events...), second.Events...) {
		if seen[e.ID] {
			t.Fatalf("event %s appeared on two pages", e.ID)
		}
		seen[e.ID] = true
	}
	// And ordering continues across the boundary.
	if !first.Events[9].ChangedAt.After(second.Events[0].ChangedAt) {
		t.Error("page 2 should continue after page 1, newest first")
	}
}

func TestListDefaultsToFivePerPage(t *testing.T) {
	ctx := context.Background()
	mem, _, uid := fixture(t)
	h := service.NewHistory(mem, 7)
	now := time.Now()
	h.SetClock(func() time.Time { return now })
	seedEvents(mem, uid, now, 30)

	got, err := h.List(ctx, uid, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.PerPage != service.DefaultPageSize || len(got.Events) != service.DefaultPageSize {
		t.Errorf("per_page = %d with %d events, want %d",
			got.PerPage, len(got.Events), service.DefaultPageSize)
	}
}

// A stale link to a page past the end is an empty list, not an error.
func TestListPastTheEndIsEmpty(t *testing.T) {
	ctx := context.Background()
	mem, _, uid := fixture(t)
	h := service.NewHistory(mem, 7)
	now := time.Now()
	h.SetClock(func() time.Time { return now })
	seedEvents(mem, uid, now, 3)

	page, size := 9, 10
	got, err := h.List(ctx, uid, &page, &size)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Events) != 0 {
		t.Errorf("expected no events, got %d", len(got.Events))
	}
	if got.Total != 3 {
		t.Errorf("total should still be reported, got %d", got.Total)
	}
}

func TestListEmptyHistoryStillReportsOnePage(t *testing.T) {
	mem, _, uid := fixture(t)
	h := service.NewHistory(mem, 7)

	got, err := h.List(context.Background(), uid, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Total != 0 || got.TotalPages != 1 || len(got.Events) != 0 {
		t.Errorf("unexpected empty page: %+v", got)
	}
}

func TestListClampsAnAbsurdPageSize(t *testing.T) {
	ctx := context.Background()
	mem, _, uid := fixture(t)
	h := service.NewHistory(mem, 7)
	now := time.Now()
	h.SetClock(func() time.Time { return now })
	seedEvents(mem, uid, now, 150)

	huge := 10000
	got, err := h.List(ctx, uid, nil, &huge)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.PerPage != service.MaxPageSize || len(got.Events) != service.MaxPageSize {
		t.Errorf("per_page = %d with %d events, want it clamped to %d",
			got.PerPage, len(got.Events), service.MaxPageSize)
	}
}

// Paging must not reach outside the retention window.
func TestPaginationRespectsTheWindow(t *testing.T) {
	ctx := context.Background()
	mem, _, uid := fixture(t)
	h := service.NewHistory(mem, 7)
	now := time.Now()
	h.SetClock(func() time.Time { return now })

	seedEvents(mem, uid, now, 5)
	old := "old"
	mem.SeedChange(models.ProfileChange{
		UserID: uid, Field: models.FieldMobile, OldValue: &old, NewValue: &old,
		ChangedBy: uid, ChangedAt: now.AddDate(0, 0, -30),
	})

	got, err := h.List(ctx, uid, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Total != 5 {
		t.Errorf("total = %d, want 5 — the 30-day-old event is outside the window", got.Total)
	}
}
