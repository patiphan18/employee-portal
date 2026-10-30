-- Employee Portal schema. See docs/DESIGN_DOC.md section 2.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL,
    password_hash text        NOT NULL,
    first_name    text        NOT NULL,
    last_name     text        NOT NULL,
    mobile        text,
    address       text,
    avatar_url    text,
    role          text        NOT NULL DEFAULT 'employee',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_role_check CHECK (role IN ('employee', 'admin'))
);

-- Case-insensitive uniqueness without requiring the citext extension (FR-3.5).
CREATE UNIQUE INDEX IF NOT EXISTS users_email_key ON users (lower(email));

CREATE TABLE IF NOT EXISTS skills (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL,
    category   text        NOT NULL DEFAULT 'General',
    active     boolean     NOT NULL DEFAULT true,
    sort_order integer     NOT NULL DEFAULT 100,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS skills_name_key ON skills (lower(name));
CREATE INDEX IF NOT EXISTS skills_active_idx ON skills (active, sort_order);

CREATE TABLE IF NOT EXISTS user_skills (
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    skill_id   uuid        NOT NULL REFERENCES skills (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, skill_id)
);

CREATE INDEX IF NOT EXISTS user_skills_skill_idx ON user_skills (skill_id);

-- Append-only audit log (FR-5.6).
CREATE TABLE IF NOT EXISTS profile_changes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    field      text        NOT NULL,
    old_value  text,
    new_value  text,
    changed_by uuid        NOT NULL REFERENCES users (id),
    changed_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT profile_changes_field_check
        CHECK (field IN ('avatar', 'mobile', 'email', 'address', 'skills'))
);

-- The history query reads a prefix of this index (FR-5.2).
CREATE INDEX IF NOT EXISTS profile_changes_user_time_idx
    ON profile_changes (user_id, changed_at DESC);

CREATE TABLE IF NOT EXISTS settings (
    key        text PRIMARY KEY,
    value      text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
