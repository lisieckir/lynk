-- Users gain roles, disabled state, and session invalidation markers.
-- Applied transactionally by database.Migrate. Do not edit applied migrations.
--
-- Existing databases: the first user by lowest id becomes admin.
-- New users default to role 'user' and disabled = 0.

ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user'
    CHECK (role IN ('user', 'admin'));
ALTER TABLE users ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0
    CHECK (disabled IN (0, 1));
ALTER TABLE users ADD COLUMN disabled_at TEXT;
ALTER TABLE users ADD COLUMN session_valid_after TEXT;

UPDATE users
SET role = 'admin'
WHERE id = (SELECT MIN(id) FROM users WHERE id IS NOT NULL);

-- If there are no users yet, the UPDATE above affects no rows.
