package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/digithun/employee-portal/backend/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Postgres implements Store on database/sql. Every query is parameterised (NFR-3).
type Postgres struct{ db *sql.DB }

func NewPostgres(db *sql.DB) *Postgres { return &Postgres{db: db} }

// Open dials the database and waits for it to accept connections.
func Open(ctx context.Context, dsn string, wait time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	deadline := time.Now().Add(wait)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}
		if time.Now().After(deadline) {
			db.Close()
			return nil, fmt.Errorf("database unreachable after %s: %w", wait, err)
		}
		select {
		case <-ctx.Done():
			db.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (p *Postgres) Ping(ctx context.Context) error { return p.db.PingContext(ctx) }

const userColumns = `id, email, password_hash, first_name, last_name, mobile, address,
	avatar_url, role, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.Mobile, &u.Address, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (p *Postgres) UserByID(ctx context.Context, id string) (*models.User, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	return scanUser(p.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (p *Postgres) UserByEmail(ctx context.Context, email string) (*models.User, error) {
	return scanUser(p.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`,
		strings.TrimSpace(email)))
}

// ListUsers returns a stable, paginated view of accounts for the admin console.
func (p *Postgres) ListUsers(ctx context.Context, limit, offset int) ([]models.User, int, error) {
	var total int
	if err := p.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []models.User{}, total, nil
	}
	rows, err := p.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users
		ORDER BY created_at DESC, lower(last_name), lower(first_name), lower(email), id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	users := make([]models.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *u)
	}
	return users, total, rows.Err()
}

func (p *Postgres) CreateManagedUser(ctx context.Context, u *models.User) error {
	err := p.db.QueryRowContext(ctx, `INSERT INTO users
		(email, password_hash, first_name, last_name, mobile, address, role)
		VALUES (lower($1), $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		u.Email, u.PasswordHash, u.FirstName, u.LastName, u.Mobile, u.Address, u.Role,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if isUniqueViolation(err, "users_email_key") {
		return ErrEmailTaken
	}
	return err
}

// UpdateManagedUser serializes role changes so concurrent requests cannot remove the
// final administrator. The lock is held only for this short account mutation.
func (p *Postgres) UpdateManagedUser(ctx context.Context, u *models.User) error {
	err := p.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE`); err != nil {
			return err
		}
		var currentRole string
		if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = $1 FOR UPDATE`, u.ID).Scan(&currentRole); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if currentRole == models.RoleAdmin && u.Role != models.RoleAdmin {
			var admins int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role = $1`, models.RoleAdmin).Scan(&admins); err != nil {
				return err
			}
			if admins <= 1 {
				return ErrLastAdmin
			}
		}
		var result sql.Result
		var err error
		if u.PasswordHash == "" {
			result, err = tx.ExecContext(ctx, `UPDATE users SET email = lower($2), first_name = $3,
				last_name = $4, mobile = $5, address = $6, role = $7, updated_at = now() WHERE id = $1`,
				u.ID, u.Email, u.FirstName, u.LastName, u.Mobile, u.Address, u.Role)
		} else {
			result, err = tx.ExecContext(ctx, `UPDATE users SET email = lower($2), first_name = $3,
				last_name = $4, mobile = $5, address = $6, role = $7, password_hash = $8,
				updated_at = now() WHERE id = $1`,
				u.ID, u.Email, u.FirstName, u.LastName, u.Mobile, u.Address, u.Role, u.PasswordHash)
		}
		if err != nil {
			if isUniqueViolation(err, "users_email_key") {
				return ErrEmailTaken
			}
			return err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
	return err
}

func (p *Postgres) DeleteUser(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrNotFound
	}
	return p.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE`); err != nil {
			return err
		}
		var role string
		if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = $1 FOR UPDATE`, id).Scan(&role); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if role == models.RoleAdmin {
			var admins int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role = $1`, models.RoleAdmin).Scan(&admins); err != nil {
				return err
			}
			if admins <= 1 {
				return ErrLastAdmin
			}
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
		return err
	})
}

func (p *Postgres) UpdateUser(ctx context.Context, u *models.User, changes []models.ProfileChange) error {
	return p.inTx(ctx, func(tx *sql.Tx) error {
		// first_name and last_name are intentionally absent: HR owns them (FR-3.1).
		res, err := tx.ExecContext(ctx, `
			UPDATE users
			   SET email = lower($2), mobile = $3, address = $4, avatar_url = $5,
			       updated_at = now()
			 WHERE id = $1`,
			u.ID, u.Email, u.Mobile, u.Address, u.AvatarURL)
		if err != nil {
			if isUniqueViolation(err, "users_email_key") {
				return ErrEmailTaken
			}
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return insertChanges(ctx, tx, changes)
	})
}

func (p *Postgres) ListSkills(ctx context.Context, activeOnly bool) ([]models.Skill, error) {
	q := `SELECT id, name, active, sort_order, created_at FROM skills`
	if activeOnly {
		q += ` WHERE active = true`
	}
	q += ` ORDER BY sort_order, name`
	rows, err := p.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Skill, 0)
	for rows.Next() {
		var s models.Skill
		if err := rows.Scan(&s.ID, &s.Name, &s.Active, &s.SortOrder, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (p *Postgres) SkillByID(ctx context.Context, id string) (*models.Skill, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	var s models.Skill
	err := p.db.QueryRowContext(ctx,
		`SELECT id, name, active, sort_order, created_at FROM skills WHERE id = $1`, id).
		Scan(&s.ID, &s.Name, &s.Active, &s.SortOrder, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (p *Postgres) CreateSkill(ctx context.Context, s *models.Skill) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	err := p.db.QueryRowContext(ctx, `
		INSERT INTO skills (id, name, active, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`,
		s.ID, s.Name, s.Active, s.SortOrder).Scan(&s.CreatedAt)
	if isUniqueViolation(err, "skills_name_key") {
		return ErrConflict
	}
	return err
}

func (p *Postgres) UpdateSkill(ctx context.Context, s *models.Skill) error {
	res, err := p.db.ExecContext(ctx, `
		UPDATE skills SET name = $2, active = $3, sort_order = $4
		 WHERE id = $1`,
		s.ID, s.Name, s.Active, s.SortOrder)
	if err != nil {
		if isUniqueViolation(err, "skills_name_key") {
			return ErrConflict
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) UserSkills(ctx context.Context, userID string) ([]models.Skill, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrNotFound
	}
	// Inactive skills stay visible once selected (FR-4.3), so there is no active filter.
	rows, err := p.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.active, s.sort_order, s.created_at
		  FROM user_skills us
		  JOIN skills s ON s.id = us.skill_id
		 WHERE us.user_id = $1
		 ORDER BY s.sort_order, s.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Skill, 0)
	for rows.Next() {
		var s models.Skill
		if err := rows.Scan(&s.ID, &s.Name, &s.Active, &s.SortOrder, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UserCustomSkills returns this employee's own free-text skills (FR-3.10).
func (p *Postgres) UserCustomSkills(ctx context.Context, userID string) ([]string, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrNotFound
	}
	rows, err := p.db.QueryContext(ctx,
		`SELECT name FROM user_custom_skills WHERE user_id = $1 ORDER BY lower(name)`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (p *Postgres) SetUserSkills(ctx context.Context, userID string, skillIDs, customSkills []string, changes []models.ProfileChange) error {
	return p.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_skills WHERE user_id = $1`, userID); err != nil {
			return err
		}
		for _, id := range skillIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO user_skills (user_id, skill_id) VALUES ($1, $2)`, userID, id); err != nil {
				if isForeignKeyViolation(err) {
					return ErrNotFound
				}
				return err
			}
		}

		if _, err := tx.ExecContext(ctx,
			`DELETE FROM user_custom_skills WHERE user_id = $1`, userID); err != nil {
			return err
		}
		for _, name := range customSkills {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO user_custom_skills (user_id, name) VALUES ($1, $2)`, userID, name); err != nil {
				if isForeignKeyViolation(err) {
					return ErrNotFound
				}
				return err
			}
		}
		return insertChanges(ctx, tx, changes)
	})
}

func (p *Postgres) History(ctx context.Context, userID string, since time.Time, limit, offset int) ([]models.ProfileChange, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrNotFound
	}
	// A NULL limit means "all rows", which keeps one query for both cases.
	var limitArg any
	if limit > 0 {
		limitArg = limit
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, user_id, field, old_value, new_value, changed_by, changed_at
		  FROM profile_changes
		 WHERE user_id = $1 AND changed_at > $2
		 ORDER BY changed_at DESC, id DESC
		 LIMIT $3 OFFSET $4`, userID, since, limitArg, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ProfileChange, 0)
	for rows.Next() {
		var c models.ProfileChange
		if err := rows.Scan(&c.ID, &c.UserID, &c.Field, &c.OldValue, &c.NewValue, &c.ChangedBy, &c.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Postgres) HistoryCount(ctx context.Context, userID string, since time.Time) (int, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return 0, ErrNotFound
	}
	var n int
	err := p.db.QueryRowContext(ctx, `
		SELECT count(*) FROM profile_changes
		 WHERE user_id = $1 AND changed_at > $2`, userID, since).Scan(&n)
	return n, err
}

func (p *Postgres) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := p.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return v, err
}

func (p *Postgres) SetSetting(ctx context.Context, key, value string) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, value)
	return err
}

func insertChanges(ctx context.Context, tx *sql.Tx, changes []models.ProfileChange) error {
	for _, c := range changes {
		if c.ID == "" {
			c.ID = uuid.NewString()
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO profile_changes (id, user_id, field, old_value, new_value, changed_by)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			c.ID, c.UserID, c.Field, c.OldValue, c.NewValue, c.ChangedBy); err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func isUniqueViolation(err error, constraint string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return pqErr.Code == "23505" && (constraint == "" || pqErr.Constraint == constraint)
}

func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23503"
}

// InsertUser creates a user. It exists for the demo seeder in cmd/server; the portal
// itself never creates accounts (HR owns that).
func (p *Postgres) InsertUser(ctx context.Context, email, passwordHash, first, last, role, mobile string) error {
	var mobilePtr *string
	if strings.TrimSpace(mobile) != "" {
		mobilePtr = &mobile
	}
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, role, mobile)
		VALUES (lower($1), $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING`,
		email, passwordHash, first, last, role, mobilePtr)
	return err
}
