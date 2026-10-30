-- Idempotent seed data (FR-4.4). Demo users are seeded from Go so that passwords are
-- bcrypt-hashed by the same code path that verifies them.

INSERT INTO settings (key, value) VALUES ('history_retention_days', '7')
ON CONFLICT (key) DO NOTHING;

INSERT INTO skills (name, sort_order) VALUES
    ('Go', 10),
    ('Node.js', 20),
    ('Python', 30),
    ('React', 40),
    ('Vue', 50),
    ('TypeScript', 60),
    ('Tailwind CSS', 70),
    ('Flutter', 80),
    ('Swift', 90),
    ('Kotlin', 100),
    ('CI/CD', 110),
    ('Docker', 120),
    ('Kubernetes', 130),
    ('Terraform', 140),
    ('PostgreSQL', 150),
    ('Redis', 160),
    ('UI/UX Design', 170),
    ('Figma', 180),
    ('Scrum', 190),
    ('Technical Writing', 200)
ON CONFLICT DO NOTHING;
