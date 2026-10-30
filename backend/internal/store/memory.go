package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/google/uuid"
)

// Memory is a complete in-memory Store. It backs the handler tests and the
// `--memory` demo mode, so the whole suite runs without Postgres.
type Memory struct {
	mu         sync.RWMutex
	users      map[string]*models.User
	skills     map[string]*models.Skill
	userSkills map[string][]string
	customs    map[string][]string
	changes    []models.ProfileChange
	settings   map[string]string
}

func NewMemory() *Memory {
	return &Memory{
		users:      map[string]*models.User{},
		skills:     map[string]*models.Skill{},
		userSkills: map[string][]string{},
		customs:    map[string][]string{},
		settings:   map[string]string{},
	}
}

// SeedUser inserts a user directly, bypassing validation. Tests and the demo seeder use it.
func (m *Memory) SeedUser(u *models.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *u
	if cp.ID == "" {
		cp.ID = uuid.NewString()
	}
	cp.Email = strings.ToLower(cp.Email)
	m.users[cp.ID] = &cp
}

// SeedSkill inserts a catalog entry directly.
func (m *Memory) SeedSkill(s *models.Skill) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *s
	if cp.ID == "" {
		cp.ID = uuid.NewString()
	}
	m.skills[cp.ID] = &cp
}

// SeedUserSkills assigns a selection directly, without writing history.
func (m *Memory) SeedUserSkills(userID string, skillIDs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userSkills[userID] = append([]string(nil), skillIDs...)
}

// SeedChange appends an audit row directly, with a caller-chosen timestamp so that
// retention-window tests can place events in the past.
func (m *Memory) SeedChange(c models.ProfileChange) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.ChangedAt.IsZero() {
		c.ChangedAt = time.Now()
	}
	m.changes = append(m.changes, c)
	return c.ID
}

func (m *Memory) Ping(context.Context) error { return nil }

func (m *Memory) UserByID(_ context.Context, id string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *Memory) UserByEmail(_ context.Context, email string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	email = strings.ToLower(strings.TrimSpace(email))
	for _, u := range m.users {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) UpdateUser(_ context.Context, u *models.User, changes []models.ProfileChange) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.users[u.ID]
	if !ok {
		return ErrNotFound
	}
	email := strings.ToLower(strings.TrimSpace(u.Email))
	for id, other := range m.users {
		if id != u.ID && other.Email == email {
			return ErrEmailTaken
		}
	}
	// Names are HR-owned: keep the stored values whatever the caller passed (FR-3.1).
	cp := *u
	cp.Email = email
	cp.FirstName = cur.FirstName
	cp.LastName = cur.LastName
	cp.PasswordHash = cur.PasswordHash
	cp.Role = cur.Role
	cp.CreatedAt = cur.CreatedAt
	cp.UpdatedAt = time.Now()
	m.users[u.ID] = &cp
	m.appendChangesLocked(changes)
	return nil
}

func (m *Memory) appendChangesLocked(changes []models.ProfileChange) {
	now := time.Now()
	for _, c := range changes {
		if c.ID == "" {
			c.ID = uuid.NewString()
		}
		if c.ChangedAt.IsZero() {
			c.ChangedAt = now
		}
		m.changes = append(m.changes, c)
	}
}

func (m *Memory) ListSkills(_ context.Context, activeOnly bool) ([]models.Skill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]models.Skill, 0, len(m.skills))
	for _, s := range m.skills {
		if activeOnly && !s.Active {
			continue
		}
		out = append(out, *s)
	}
	sortSkills(out)
	return out, nil
}

func (m *Memory) SkillByID(_ context.Context, id string) (*models.Skill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skills[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (m *Memory) CreateSkill(_ context.Context, s *models.Skill) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.skills {
		if strings.EqualFold(existing.Name, s.Name) {
			return ErrConflict
		}
	}
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	s.CreatedAt = time.Now()
	cp := *s
	m.skills[s.ID] = &cp
	return nil
}

func (m *Memory) UpdateSkill(_ context.Context, s *models.Skill) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.skills[s.ID]
	if !ok {
		return ErrNotFound
	}
	cp := *s
	cp.CreatedAt = cur.CreatedAt
	m.skills[s.ID] = &cp
	return nil
}

func (m *Memory) UserSkills(_ context.Context, userID string) ([]models.Skill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := m.userSkills[userID]
	out := make([]models.Skill, 0, len(ids))
	for _, id := range ids {
		if s, ok := m.skills[id]; ok {
			out = append(out, *s)
		}
	}
	sortSkills(out)
	return out, nil
}

// SeedCustomSkills assigns free-text skills directly, without writing history.
func (m *Memory) SeedCustomSkills(userID string, names []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customs[userID] = append([]string(nil), names...)
}

func (m *Memory) UserCustomSkills(_ context.Context, userID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := append([]string(nil), m.customs[userID]...)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out, nil
}

func (m *Memory) SetUserSkills(_ context.Context, userID string, skillIDs, customSkills []string, changes []models.ProfileChange) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[userID]; !ok {
		return ErrNotFound
	}
	for _, id := range skillIDs {
		if _, ok := m.skills[id]; !ok {
			return ErrNotFound
		}
	}
	m.userSkills[userID] = append([]string(nil), skillIDs...)
	m.customs[userID] = append([]string(nil), customSkills...)
	m.appendChangesLocked(changes)
	return nil
}

func (m *Memory) History(_ context.Context, userID string, since time.Time, limit, offset int) ([]models.ProfileChange, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Walk backwards so that the most recently appended event comes first, then sort
	// stably: events written in the same clock tick keep insertion order, newest first.
	out := make([]models.ProfileChange, 0)
	for i := len(m.changes) - 1; i >= 0; i-- {
		if c := m.changes[i]; c.UserID == userID && c.ChangedAt.After(since) {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ChangedAt.After(out[j].ChangedAt) })

	if offset < 0 {
		offset = 0
	}
	if offset >= len(out) {
		return []models.ProfileChange{}, nil
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) HistoryCount(_ context.Context, userID string, since time.Time) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, c := range m.changes {
		if c.UserID == userID && c.ChangedAt.After(since) {
			n++
		}
	}
	return n, nil
}

func (m *Memory) Setting(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.settings[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *Memory) SetSetting(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings[key] = value
	return nil
}

func sortSkills(s []models.Skill) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].SortOrder != s[j].SortOrder {
			return s[i].SortOrder < s[j].SortOrder
		}
		return s[i].Name < s[j].Name
	})
}
