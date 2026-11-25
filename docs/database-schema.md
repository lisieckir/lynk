# Lynk database schema proposal

Status: Phase 0 design; schema is not implemented yet.

The schema is intentionally internal. API serializers will map it to the
Linkding-compatible DTO documented in `docs/api-compatibility.md`.

## Tables

### `users`

| Column | Type | Constraints |
|---|---|---|
| `id` | INTEGER | primary key |
| `username` | TEXT | unique, not null |
| `password_hash` | TEXT | not null for local login |
| `created_at` | TEXT | UTC timestamp, not null |
| `updated_at` | TEXT | UTC timestamp, not null |

Single-user MVP deployments still use this table so API tokens and ownership
are explicit. Multi-user UI is not required before the MVP, but ownership
must not be hard-coded.

### `api_tokens`

| Column | Type | Constraints |
|---|---|---|
| `id` | INTEGER | primary key |
| `user_id` | INTEGER | foreign key to `users`, cascade delete |
| `token_digest` | BLOB/TEXT | unique, not null |
| `token_prefix` | TEXT | non-secret display/lookup aid |
| `name` | TEXT | not null |
| `created_at` | TEXT | UTC timestamp |
| `revoked_at` | TEXT | nullable |
| `last_used_at` | TEXT | nullable |

The raw token is never stored. Token creation returns the raw value once.
Lookup uses the digest and only non-revoked rows.

### `user_profiles`

Store the profile fields needed by the compatible response and UI:
`theme`, `bookmark_date_display`, `bookmark_link_target`,
`web_archive_integration`, `tag_search`, `enable_sharing`,
`enable_public_sharing`, `enable_favicons`, `display_url`,
`permanent_notes`, and JSON `search_preferences`. Add `version` as an API
computed field, not a mutable database fact.

`user_id` is the primary key and foreign key to `users`.

### `bookmarks`

| Column | Type | Constraints |
|---|---|---|
| `id` | INTEGER | primary key |
| `user_id` | INTEGER | foreign key, not null |
| `url` | TEXT | not null |
| `url_normalized` | TEXT | not null |
| `title` | TEXT | not null/default empty |
| `description` | TEXT | not null/default empty |
| `notes` | TEXT | not null/default empty |
| `is_archived` | INTEGER | boolean, not null/default false |
| `unread` | INTEGER | boolean, not null/default false |
| `shared` | INTEGER | boolean, not null/default false |
| `web_archive_snapshot_url` | TEXT | nullable |
| `favicon_url` or asset reference | TEXT | nullable |
| `preview_image_url` or asset reference | TEXT | nullable |
| `date_added` | TEXT | UTC RFC3339, not null |
| `date_modified` | TEXT | UTC RFC3339, not null |

Create a unique index on `(user_id, url_normalized)`. Duplicate POST must
merge/update this row, while PUT/PATCH URL collision must return a validation
error as Linkding does.

`website_title` and `website_description` are not required storage columns:
the compatibility DTO emits them as `null` unless a later approved feature
requires different semantics.

### `tags`

| Column | Type | Constraints |
|---|---|---|
| `id` | INTEGER | primary key |
| `user_id` | INTEGER | foreign key, not null |
| `name` | TEXT | not null |
| `date_added` | TEXT | UTC timestamp |

Unique index `(user_id, name)` should use the chosen Linkding-compatible
case/normalization policy. The exact policy is an open decision to verify
against upstream tests before implementation.

### `bookmark_tags`

`bookmark_id`, `tag_id` composite primary key, both foreign keys with cascade
delete. Tag assignment is replacement semantics when `tag_names` is supplied
on update, matching the serializer.

### `bookmark_search`

FTS5 external-content table keyed by bookmark row ID, with indexed columns:
`url`, `title`, `description`, `notes`, and `tags`. The user ID and archived
state remain canonical in `bookmarks`; do not duplicate authorization state in
the search index unless needed for query performance.

Use triggers or one transaction in the bookmark service to rebuild the
denormalized `tags` text. Migrations must include an explicit rebuild path.

## Indexes

* `bookmarks(user_id, is_archived, date_modified DESC)`;
* `bookmarks(user_id, date_added DESC)`;
* unique `bookmarks(user_id, url_normalized)`;
* unique `tags(user_id, name)`;
* `bookmark_tags(tag_id, bookmark_id)`;
* `api_tokens(token_digest)` and a filtered/checked path for revocation.

Add indexes only after query plans/benchmarks demonstrate need; SQLite's
small footprint is a priority.

## Transaction rules

* create/update bookmark, tag reconciliation and search synchronization are
  one transaction;
* archive/unarchive updates `is_archived` and `date_modified` atomically;
* revoke token is a single transaction;
* migrations execute in order and record their version;
* readers must use context-aware queries and never hold a transaction during
  network metadata fetching.

## SQLite pragmas and migration requirements

At connection initialization verify and configure:

```sql
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
```

The exact pool settings and cache size are implementation decisions to measure.
Migrations must be idempotently tracked, run before serving traffic, and
never drop/overwrite user data during normal startup.

Phase 1 smoke tests must confirm that the selected driver actually enables
FTS5 and persists WAL mode on the target Linux build.
