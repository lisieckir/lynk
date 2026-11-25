-- Lynk initial schema. Derived from docs/database-schema.md.
-- Applied in one transaction by internal/database.Migrate.

CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE api_tokens (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_digest BLOB NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    name         TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    revoked_at   TEXT,
    last_used_at TEXT
);

CREATE INDEX api_tokens_user_revoked_idx ON api_tokens(user_id, revoked_at);

CREATE TABLE user_profiles (
    user_id                 INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    theme                   TEXT NOT NULL DEFAULT '',
    bookmark_date_display   TEXT NOT NULL DEFAULT '',
    bookmark_link_target    TEXT NOT NULL DEFAULT '',
    web_archive_integration TEXT NOT NULL DEFAULT '',
    tag_search              INTEGER NOT NULL DEFAULT 0,
    enable_sharing          INTEGER NOT NULL DEFAULT 0,
    enable_public_sharing   INTEGER NOT NULL DEFAULT 0,
    enable_favicons         INTEGER NOT NULL DEFAULT 1,
    display_url             INTEGER NOT NULL DEFAULT 0,
    permanent_notes         TEXT NOT NULL DEFAULT '',
    search_preferences      TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE bookmarks (
    id                       INTEGER PRIMARY KEY,
    user_id                  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url                      TEXT NOT NULL,
    url_normalized           TEXT NOT NULL,
    title                    TEXT NOT NULL DEFAULT '',
    description              TEXT NOT NULL DEFAULT '',
    notes                    TEXT NOT NULL DEFAULT '',
    -- Denormalized tag text used by the FTS5 index; rebuilt from bookmark_tags
    -- by the bookmark service rather than stored as canonical tag data.
    tags                     TEXT NOT NULL DEFAULT '',
    is_archived              INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0, 1)),
    unread                   INTEGER NOT NULL DEFAULT 0 CHECK (unread IN (0, 1)),
    shared                   INTEGER NOT NULL DEFAULT 0 CHECK (shared IN (0, 1)),
    web_archive_snapshot_url TEXT,
    favicon_url              TEXT,
    preview_image_url        TEXT,
    date_added               TEXT NOT NULL,
    date_modified            TEXT NOT NULL
);

CREATE UNIQUE INDEX bookmarks_user_url_idx ON bookmarks(user_id, url_normalized);
CREATE INDEX bookmarks_user_archived_modified_idx ON bookmarks(user_id, is_archived, date_modified DESC);
CREATE INDEX bookmarks_user_added_idx ON bookmarks(user_id, date_added DESC);

CREATE TABLE tags (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    date_added TEXT NOT NULL
);

-- Exact name comparison; case normalization is deferred to the Linkding
-- compatibility research decision recorded in docs/database-schema.md.
CREATE UNIQUE INDEX tags_user_name_idx ON tags(user_id, name);

CREATE TABLE bookmark_tags (
    bookmark_id INTEGER NOT NULL REFERENCES bookmarks(id) ON DELETE CASCADE,
    tag_id      INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (bookmark_id, tag_id)
);

CREATE INDEX bookmark_tags_tag_idx ON bookmark_tags(tag_id, bookmark_id);

CREATE VIRTUAL TABLE bookmark_search USING fts5(
    url,
    title,
    description,
    notes,
    tags,
    content='bookmarks',
    content_rowid='id'
);

CREATE TRIGGER bookmarks_ai AFTER INSERT ON bookmarks BEGIN
    INSERT INTO bookmark_search(rowid, url, title, description, notes, tags)
    VALUES (new.id, new.url, new.title, new.description, new.notes, new.tags);
END;

CREATE TRIGGER bookmarks_ad AFTER DELETE ON bookmarks BEGIN
    INSERT INTO bookmark_search(bookmark_search, rowid, url, title, description, notes, tags)
    VALUES ('delete', old.id, old.url, old.title, old.description, old.notes, old.tags);
END;

CREATE TRIGGER bookmarks_au AFTER UPDATE ON bookmarks BEGIN
    INSERT INTO bookmark_search(bookmark_search, rowid, url, title, description, notes, tags)
    VALUES ('delete', old.id, old.url, old.title, old.description, old.notes, old.tags);
    INSERT INTO bookmark_search(rowid, url, title, description, notes, tags)
    VALUES (new.id, new.url, new.title, new.description, new.notes, new.tags);
END;
