-- Add per-user session epoch to invalidate sessions on security-sensitive actions.
-- Incrementing epoch invalidates all existing sessions for that user.
ALTER TABLE users ADD COLUMN session_epoch INTEGER NOT NULL DEFAULT 0;
