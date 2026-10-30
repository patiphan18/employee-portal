-- Skills no longer carry a category: the brief only asked for a configurable list
-- of skills to pick from, so the grouping was removed.
ALTER TABLE skills DROP COLUMN IF EXISTS category;

-- An employee may add a skill that is not in the catalog. It belongs to that employee
-- only and is never offered to anyone else, so it lives here rather than in `skills`.
CREATE TABLE IF NOT EXISTS user_custom_skills (
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, name)
);

-- One employee cannot hold "Elixir" and "elixir" at the same time.
CREATE UNIQUE INDEX IF NOT EXISTS user_custom_skills_name_key
    ON user_custom_skills (user_id, lower(name));
