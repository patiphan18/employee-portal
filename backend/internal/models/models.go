package models

import "time"

// Roles recognised by the authorisation middleware.
const (
	RoleEmployee = "employee"
	RoleAdmin    = "admin"
)

// Change field identifiers stored in profile_changes.field.
const (
	FieldAvatar  = "avatar"
	FieldMobile  = "mobile"
	FieldEmail   = "email"
	FieldAddress = "address"
	FieldSkills  = "skills"
)

// User is the employee record. FirstName and LastName are owned by HR and are never
// written by the portal (FR-3.1).
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Mobile       *string   `json:"mobile"`
	Address      *string   `json:"address"`
	AvatarURL    *string   `json:"avatar_url"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Skill is a catalog entry an administrator maintains.
type Skill struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// ProfileSkill is one skill as it appears on an employee's profile: either a catalog
// entry or a free-text skill that employee added for themselves (FR-3.10). A custom
// skill has no ID, because it exists only on that one profile.
type ProfileSkill struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
}

// ProfileChange is one append-only audit row (FR-5.1).
type ProfileChange struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Field     string    `json:"field"`
	OldValue  *string   `json:"old_value"`
	NewValue  *string   `json:"new_value"`
	ChangedBy string    `json:"changed_by"`
	ChangedAt time.Time `json:"changed_at"`
}

// Profile is the API view of a user: the stored record plus derived data.
type Profile struct {
	ID           string         `json:"id"`
	FirstName    string         `json:"first_name"`
	LastName     string         `json:"last_name"`
	Email        string         `json:"email"`
	Mobile       *string        `json:"mobile"`
	Address      *string        `json:"address"`
	AvatarURL    *string        `json:"avatar_url"`
	Role         string         `json:"role"`
	Skills       []ProfileSkill `json:"skills"`
	Completeness Completeness   `json:"completeness"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// Completeness drives the progress meter (FR-7).
type Completeness struct {
	Percent int      `json:"percent"`
	Missing []string `json:"missing"`
}

// ContactPatch carries a partial contact update. A nil pointer means "key absent, leave
// alone"; a pointer to "" means "clear this field" (FR-3.3).
type ContactPatch struct {
	Mobile  *string `json:"mobile"`
	Email   *string `json:"email"`
	Address *string `json:"address"`
}

// IsEmpty reports whether the patch carries no fields at all.
func (p ContactPatch) IsEmpty() bool {
	return p.Mobile == nil && p.Email == nil && p.Address == nil
}
