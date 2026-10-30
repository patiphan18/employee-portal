package service

import (
	"context"
	"strconv"
	"time"

	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/digithun/employee-portal/backend/internal/store"
)

const (
	MinRetentionDays = 1
	MaxRetentionDays = 365

	DefaultPageSize = 5
	MaxPageSize     = 100
)

// History reads the audit log. It is read-only: the log records what happened and
// nothing in the application rewrites or replays it.
type History struct {
	store       store.Store
	defaultDays int // from HISTORY_RETENTION_DAYS
	now         func() time.Time
}

func NewHistory(s store.Store, defaultDays int) *History {
	return &History{store: s, defaultDays: defaultDays, now: time.Now}
}

// SetClock replaces the time source. Tests use it to age events.
func (h *History) SetClock(f func() time.Time) { h.now = f }

// ResolveDays applies the precedence in FR-5.4: the settings table an administrator
// controls, then the env default, then 7 — clamped to [1, 365].
//
// Employees have no say in this. The window is a policy decision about how far back
// the portal exposes an audit trail, so it is set once for everyone rather than
// chosen per request.
func (h *History) ResolveDays(ctx context.Context) int {
	if v, err := h.store.Setting(ctx, store.SettingHistoryRetentionDays); err == nil {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			return ClampDays(n)
		}
	}
	return ClampDays(h.defaultDays)
}

// ClampDays keeps a window inside the supported range, falling back to 7 for values
// that make no sense at all.
func ClampDays(days int) int {
	if days < MinRetentionDays {
		if days <= 0 {
			return 7
		}
		return MinRetentionDays
	}
	if days > MaxRetentionDays {
		return MaxRetentionDays
	}
	return days
}

// ClampPageSize keeps a page size sensible, so one request cannot ask for the whole
// log at once.
func ClampPageSize(size int) int {
	if size <= 0 {
		return DefaultPageSize
	}
	if size > MaxPageSize {
		return MaxPageSize
	}
	return size
}

// Page describes one page of the audit log (FR-5.8).
type Page struct {
	Days       int                    `json:"days"`
	Page       int                    `json:"page"`
	PerPage    int                    `json:"per_page"`
	Total      int                    `json:"total"`
	TotalPages int                    `json:"total_pages"`
	Events     []models.ProfileChange `json:"events"`
}

// List returns one page of the caller's events, newest first, together with the
// window and paging actually used. Asking for a page past the end returns an empty
// list rather than an error: it is a stale link, not a failure.
func (h *History) List(ctx context.Context, userID string, page, perPage *int) (*Page, error) {
	days := h.ResolveDays(ctx)
	since := h.now().AddDate(0, 0, -days)

	size := DefaultPageSize
	if perPage != nil {
		size = ClampPageSize(*perPage)
	}
	current := 1
	if page != nil && *page > 1 {
		current = *page
	}

	total, err := h.store.HistoryCount(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	totalPages := (total + size - 1) / size
	if totalPages < 1 {
		totalPages = 1
	}

	events, err := h.store.History(ctx, userID, since, size, (current-1)*size)
	if err != nil {
		return nil, err
	}

	return &Page{
		Days:       days,
		Page:       current,
		PerPage:    size,
		Total:      total,
		TotalPages: totalPages,
		Events:     events,
	}, nil
}
