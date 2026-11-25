-- Keep the denormalized bookmark tag text and the FTS5 index in sync when
-- bookmark_tags changes. This migration does not modify applied 0001.
--
-- The denormalized text is recomputed from the canonical bookmark_tags rows in
-- a deterministic order: tag name (BINARY collation), then tag id as a
-- tie-breaker. LIMIT -1 stops SQLite from flattening the ordered subquery into
-- the aggregate, so group_concat observes that order.
--
-- These triggers only UPDATE bookmarks.tags. The bookmarks_au trigger from
-- 0001 then performs exactly one FTS delete+insert pair for that bookmark.
-- Nothing here writes to bookmark_search directly, so repeated tag inserts,
-- deletions, replacements and updates cannot create duplicate FTS rows.

CREATE TRIGGER bookmark_tags_ai AFTER INSERT ON bookmark_tags BEGIN
    UPDATE bookmarks
    SET tags = COALESCE((
        SELECT group_concat(ordered.name, ' ')
        FROM (
            SELECT t.name AS name
            FROM tags AS t
            JOIN bookmark_tags AS bt ON bt.tag_id = t.id
            WHERE bt.bookmark_id = NEW.bookmark_id
            ORDER BY t.name COLLATE BINARY, t.id
            LIMIT -1
        ) AS ordered
    ), '')
    WHERE id = NEW.bookmark_id;
END;

CREATE TRIGGER bookmark_tags_ad AFTER DELETE ON bookmark_tags BEGIN
    UPDATE bookmarks
    SET tags = COALESCE((
        SELECT group_concat(ordered.name, ' ')
        FROM (
            SELECT t.name AS name
            FROM tags AS t
            JOIN bookmark_tags AS bt ON bt.tag_id = t.id
            WHERE bt.bookmark_id = OLD.bookmark_id
            ORDER BY t.name COLLATE BINARY, t.id
            LIMIT -1
        ) AS ordered
    ), '')
    WHERE id = OLD.bookmark_id;
END;

CREATE TRIGGER bookmark_tags_au AFTER UPDATE ON bookmark_tags BEGIN
    UPDATE bookmarks
    SET tags = COALESCE((
        SELECT group_concat(ordered.name, ' ')
        FROM (
            SELECT t.name AS name
            FROM tags AS t
            JOIN bookmark_tags AS bt ON bt.tag_id = t.id
            WHERE bt.bookmark_id = NEW.bookmark_id
            ORDER BY t.name COLLATE BINARY, t.id
            LIMIT -1
        ) AS ordered
    ), '')
    WHERE id = NEW.bookmark_id;

    -- A row can be moved to another bookmark; refresh the old parent too.
    UPDATE bookmarks
    SET tags = COALESCE((
        SELECT group_concat(ordered.name, ' ')
        FROM (
            SELECT t.name AS name
            FROM tags AS t
            JOIN bookmark_tags AS bt ON bt.tag_id = t.id
            WHERE bt.bookmark_id = OLD.bookmark_id
            ORDER BY t.name COLLATE BINARY, t.id
            LIMIT -1
        ) AS ordered
    ), '')
    WHERE id = OLD.bookmark_id
      AND OLD.bookmark_id <> NEW.bookmark_id;
END;

-- A tag rename changes the denormalized text without touching bookmark_tags.
CREATE TRIGGER tags_au AFTER UPDATE OF name ON tags BEGIN
    UPDATE bookmarks
    SET tags = COALESCE((
        SELECT group_concat(ordered.name, ' ')
        FROM (
            SELECT t.name AS name
            FROM tags AS t
            JOIN bookmark_tags AS bt ON bt.tag_id = t.id
            WHERE bt.bookmark_id = bookmarks.id
            ORDER BY t.name COLLATE BINARY, t.id
            LIMIT -1
        ) AS ordered
    ), '')
    WHERE id IN (SELECT bookmark_id FROM bookmark_tags WHERE tag_id = NEW.id);
END;
