# Linkding API compatibility research

Status: Phase 0 research plus P2-003 API foundation implemented; P2-004
search (`q`, FTS5), limit/offset pagination and the update/archive mutations;
and P2-005 extension request replay, writable `is_archived`/`date_added`/
`date_modified` and `GET /api/tags/` pagination. All of it is verified by
`make test-compat` and `make test-integration`. Remaining unimplemented
areas: metadata fetching, asset/bundle routes and newer
Linkding search filters such as `sort`, `unread`, `shared`,
`modified_since`/`added_since`. The endpoints covered by the foundation are
listed below; metadata fetching, asset/bundle routes and newer filters
remain unimplemented and are tracked as separate board work.

## Scope and sources

The research was performed against the default branch HEADs cloned on 2026-11-25:

| Repository | Revision | Relevant files |
|---|---|---|
| `sissbruecker/linkding` | `27b7303baf41bb28babc610ac8eaa486e1ddfab5` | `bookmarks/api/routes.py`, `bookmarks/api/serializers.py`, `bookmarks/api/auth.py`, `bookmarks/settings/base.py`, `bookmarks/urls.py`, `bookmarks/services/bookmarks.py`, API tests |
| `sissbruecker/linkding-extension` | `d0221d9c5271245217d06e76c49b86a70b0fedb9` | `src/linkding.js`, `src/cache.js`, `src/profile.js`, `src/popup-form.js`, `src/background.js` |

The public API documentation at <https://linkding.link/api/> was used as a
secondary source. Source code and tests take precedence where they differ.

## Authentication

Verified in `bookmarks/api/auth.py`:

* The header is `Authorization: Token <token>` for the official extension.
* Linkding also accepts `Authorization: Bearer <token>`.
* Header keywords are case-insensitive.
* A missing/unknown scheme is not accepted by the API permission layer.
* A missing token, token with spaces, invalid characters, or unknown token is
  an authentication failure.
* The current Linkding token model generates 40 hexadecimal characters from
  `os.urandom(20)`. The token is sent as plaintext by clients.

Lynk must accept both `Token` and `Bearer` for API compatibility. Lynk should
store only a digest at rest; the clear token must be shown only at creation
time. This does not alter the wire contract.

## Compatibility matrix

| Method and URL | Auth | Query parameters | Request JSON | Response and status | Extension |
|---|---|---|---|---|---|
| `GET /api/bookmarks/` | required | `q`, `limit`, `offset`, `modified_since`, `added_since`, `bundle`; current Linkding search also understands filters such as `unread`, `shared`, `sort` | none | `200`: paginated object `{count,next,previous,results}` | Yes: search and connection test |
| `GET /api/bookmarks/archived/` | required | Same list/search parameters | none | `200`: same pagination shape, archived rows only | Not directly |
| `GET /api/bookmarks/{id}/` | required | none | none | `200`: bookmark object; `404` for inaccessible/missing row | Yes, only for older Linkding compatibility after `/check/` |
| `GET /api/bookmarks/check/` | required | required `url`; `ignore_cache=true` is accepted and ignored (Lynk keeps no metadata cache) | none | `200`: `{bookmark,metadata,auto_tags}`; `bookmark` is object or `null` | Yes, central to popup, badge and metadata |
| `POST /api/bookmarks/` | required | presence of `disable_scraping` disables scraping; presence of `disable_html_snapshot` disables snapshot scheduling | bookmark JSON | `201`: bookmark object; `400`: DRF validation object; auth/permission failures as below | Yes, with `disable_scraping`; optionally `disable_html_snapshot` |
| `PUT /api/bookmarks/{id}/` | required | none | bookmark fields; `url` is the only required field for PUT | `200`: bookmark object; `400` validation; `404` missing/inaccessible | No current extension call |
| `PATCH /api/bookmarks/{id}/` | required | none | partial bookmark fields | `200`: bookmark object; `400` validation; `404` missing/inaccessible | No current extension call |
| `DELETE /api/bookmarks/{id}/` | required | none | none | `204`, empty body; `404` missing/inaccessible | Yes |
| `POST /api/bookmarks/{id}/archive/` | required | none | none | `204`, empty body | No current extension call |
| `POST /api/bookmarks/{id}/unarchive/` | required | none | none | `204`, empty body | No current extension call |
| `GET /api/tags/` | required | `limit`, `offset` (extension uses `limit=5000`) | none | `200`: paginated `{count,next,previous,results}`; `limit`/`offset` honoured with the default page size 100 | Yes |
| `GET /api/user/profile/` | required | none | none | `200`: profile object | Yes, best-effort/background |
| `GET /api/bookmarks/{id}/assets/` and other asset routes | required | route-specific | multipart for upload | documented by Linkding; not required for current extension | No |
| `GET/POST/... /api/bundles/` | required | route-specific | bundle JSON | documented by Linkding; not required for current extension | No |

Every route above requires the trailing slash. `SimpleRouter`/Django routing
defines these URLs; Lynk should implement the slash form as canonical and
should not rely on a redirect for API clients.

## Bookmark wire format

The current `BookmarkSerializer` emits these fields:

```json
{
  "id": 1,
  "url": "https://example.com",
  "title": "Example title",
  "description": "Example description",
  "notes": "Example notes",
  "web_archive_snapshot_url": null,
  "favicon_url": null,
  "preview_image_url": null,
  "is_archived": false,
  "unread": false,
  "shared": false,
  "tag_names": ["tag1"],
  "date_added": "2020-09-26T09:46:23.006313Z",
  "date_modified": "2020-09-26T16:01:14.275335Z",
  "website_title": null,
  "website_description": null
}
```

`website_title` and `website_description` are deliberately emitted as
`null` for backwards compatibility. `favicon_url`, `preview_image_url` and
`web_archive_snapshot_url` are serialized URLs or `null`.

On create/update the client may send `url`, `title`, `description`, `notes`,
`is_archived`, `unread`, `shared`, `tag_names`, `date_added` and
`date_modified` as supported by the serializer. `tag_names` is an array of
strings. Read-only generated fields must not be accepted as authoritative
input.

### Writable archive state and dates (P2-005)

Verified in `bookmarks/api/serializers.py`: `is_archived` is an ordinary model
field and `date_added`/`date_modified` are declared as optional
`DateTimeField`s outside `read_only_fields`, so all three are writable on
create, `PUT` and `PATCH`. Lynk implements that parity:

* create persists `is_archived`, `date_added` and `date_modified` when they are
  sent; omitted values fall back to `false` and the server clock. Linkding
  fills the dates only when the client did not send them, which is how clients
  sync existing dates over the REST API, and Lynk does the same;
* `PUT`/`PATCH` persist whichever of the three fields the request carries;
  an omitted `date_added` or `is_archived` keeps its stored value and an
  omitted `date_modified` falls back to the server clock;
* values are validated before the database is touched. A malformed value is a
  field-keyed `400` keyed by `date_added`, `date_modified` or `is_archived`
  and leaves the stored row unchanged. The accepted shapes are the ISO-8601
  forms `django.utils.dateparse.parse_datetime` accepts: a `T` or space
  separator, an optional fractional second and an optional zone
  (`2000-01-01T00:00:00Z`, `2000-01-01T00:00:00.123456+02:00`,
  `2000-01-01 00:00:00`). A bare date, an empty string or a wrong JSON type is
  rejected, matching DRF;
* an omitted field or a JSON `null` means "not supplied".

Two deliberate differences from the pinned Linkding revision:

* Linkding's `update_bookmark` service restamps `date_modified` with the
  server clock on every `PUT`/`PATCH`, so a client-supplied value is validated
  and then discarded. Lynk persists a supplied `date_modified` and only uses
  the server clock when the field is omitted.
* On the duplicate-POST merge Linkding copies only `title`, `description`,
  `notes`, `unread` and `shared` into the stored row, so a submitted
  `is_archived` or `date_added` is validated but ignored there and
  `date_modified` becomes "now". Lynk matches that behaviour, which keeps an
  extension edit from un-archiving a bookmark or rewriting its creation date.

### URL normalization and validation

Verified against `bookmarks/utils.py::normalize_url` and
`bookmarks/validators.py::BookmarkURLValidator` (which inherits Django's
`URLValidator` defaults): normalization trims surrounding whitespace,
lowercases the scheme and hostname, removes trailing slashes from the path,
sorts query key/value pairs (retaining blank values and repeated keys), and
re-encodes query pairs using percent encoding with `/` left safe. It preserves
fragments, user information, path parameters and explicitly supplied ports,
including default ports. The default validator accepts `http`, `https`, `ftp`
and `ftps`; non-network schemes such as `javascript` and `data` are rejected.
The current implementation follows these rules for bookmark create/update and
uses the normalized URL within the owning user's duplicate lookup.

## Verified extension request flows

### Popup initialization

The extension:

1. loads cached profile;
2. asynchronously calls `GET /api/user/profile/`;
3. asynchronously calls `GET /api/tags/?limit=5000`;
4. calls `GET /api/bookmarks/check/?url=<encoded URL>`;
5. if the returned bookmark has no `date_added`, calls
   `GET /api/bookmarks/{id}/`;
6. uses `metadata.title` and `metadata.description` unless browser metadata is
   selected.

`check` failures are caught and treated as no server metadata, so it must not
break the popup's basic operation.

### Save

The extension sends:

```http
POST /api/bookmarks/?disable_scraping[&disable_html_snapshot]
Authorization: Token <token>
Content-Type: application/json
```

with `url`, `title`, `description`, `notes`, `tag_names`, `unread` and
`shared`. It accepts **only `201` as success** and parses the response as a
bookmark object. It treats `400` specially by displaying the serialized JSON
body and treats other statuses as errors.

Linkding's `create_bookmark` first looks up the same user's URL and merges the
incoming data into the existing bookmark. Therefore the same POST is both
create and extension edit operation. Lynk must preserve this behavior.

### Delete

The extension sends `DELETE /api/bookmarks/{id}/` and accepts **only `204`**.

### Search and omnibox

The extension sends `GET /api/bookmarks/?q=<encoded text>&limit=5` for
omnibox suggestions and accepts `200`, then reads `body.results`. It also uses
`limit=1` for connection testing and `results` must be an array.

## Pagination and search

Linkding uses DRF `LimitOffsetPagination`, default page size 100, with:

```json
{"count": 123, "next": "...", "previous": null, "results": []}
```

`next` and `previous` are absolute or request-derived URLs and may be `null`.
Lynk must preserve the keys and nullability. The extension only consumes
`results`, but API compatibility tests must verify all pagination fields.

The same envelope and rules apply to `GET /api/tags/` (P2-005): `count` is the
total number of tags the user owns, the default page size is 100, and the
extension's `limit=5000` therefore returns the whole tag set in a single page
with `next` and `previous` set to `null`.

`q` is server-side search, not a substring-only contract. Lynk's FTS query
implementation may differ internally, but must cover URL, title,
description/notes and tags and accept extension queries without requiring
extension changes.

Verified implementation differences, both covered by compatibility tests:

* Lynk tokenizes with FTS5 and ANDs the quoted tokens of `q`, so matching is
  whole-token rather than Linkding's `icontains` substring match. The final
  token is evaluated as a prefix query, which keeps the extension's omnibox
  useful while the user is still typing a partial word; an infix fragment
  (e.g. `lph` for `alpha`) does not match.
* Lynk treats a missing, unparsable or non-positive `limit` as the default
  page size of 100, so `limit=0` falls back to 100, and a missing or
  unparsable `offset` as `0`. DRF itself accepts `limit=0` as an empty page;
  Lynk normalizes that value to the default page size.

## Errors and status behavior

Verified from DRF configuration, viewsets, serializers and tests:

* unauthenticated/invalid API credentials: `401`;
* authenticated user without permission: `403` where applicable;
* invalid JSON or validation error: `400`, normally an object keyed by field;
* missing or another user's object: `404`;
* successful create: `201` with JSON;
* successful update/list/retrieve/check/profile/tags: `200` with JSON;
* successful delete/archive/unarchive: `204` with no body.

The exact DRF default error envelope is not a stable product DTO and must be
captured in compatibility tests before intentionally reproducing it. At
minimum Lynk must preserve status, JSON content type and field-keyed
validation errors used by the extension.

## Implemented API (P2-003, P2-004 and P2-005)

`internal/api/linkding` serves the canonical trailing-slash routes
`GET /api/user/profile/`, `GET /api/tags/`, `GET /api/bookmarks/check/`,
`GET/POST /api/bookmarks/`, `GET /api/bookmarks/archived/` and
`GET/PUT/PATCH/DELETE /api/bookmarks/{id}/`, plus
`POST /api/bookmarks/{id}/archive/` and `/unarchive/`. The router answers
`404` without a redirect for non-canonical paths, applies the `Token`/`Bearer`
middleware to API requests, scopes every read and write to the authenticated
user, and uses dedicated DTOs with the full Linkding bookmark field set
(generated fields remain `null` or `[]`).

List endpoints implement `q`, `limit` and `offset` and return the
`{count,next,previous,results}` envelope. `count` is the total number of
matching rows; `next` and `previous` are request-derived URLs carrying the
current limit and the adjacent offset, or `null` at the ends of the result set.
`q` is evaluated with FTS5 over the bookmark URL, title, description, notes
and tags: each run of letters/digits becomes a quoted phrase, tokens are
ANDed, and only the final token is treated as a prefix so partial words from
the extension omnibox still match. FTS metacharacters in the query are inert.
A query with no word characters matches nothing.

`GET /api/tags/` returns the owned tag list with the same
`{count,next,previous,results}` envelope, `limit`/`offset` parsing and default
page size of 100 as the bookmark lists: `count` is the total number of owned
tags, `next`/`previous` are request-derived and `null` at the ends of the
result set, and the extension's `limit=5000` loads the whole set in one page.
Check returns the documented `{bookmark,metadata,auto_tags}` shape with bounded,
null metadata, `auto_tags: []` and no network fetch; `ignore_cache` is accepted
and ignored because Lynk keeps no metadata cache. Field-keyed `400` responses,
`401`, owner-scoped `404`, `201` and empty `204` are covered by `httptest`
tests against a real migrated SQLite database; `make test-compat` runs those
tests.

### Extension request replay (P2-005)

`tests/integration/` replays the pinned extension revision's own request
sequence in-process: a real migrated SQLite database from `internal/database`
plus `httptest`, with no network, no browser and no metadata fetcher.
`make test-integration` runs the package. The replayed flows are:

| Test | Replayed requests |
|---|---|
| `TestExtensionPopupInitializationReplay` | `401` without a token, `GET /api/user/profile/`, `GET /api/tags/?limit=5000`, `GET /api/bookmarks/check/?url=...&ignore_cache=true` (and the same check without the flag), `GET /api/bookmarks/{id}/` |
| `TestExtensionSaveAndMergeReplay` | `POST /api/bookmarks/?disable_scraping&disable_html_snapshot`, then the same POST used as the extension's edit: duplicate merge, still `201`, same `id`, stored archive state and `date_added` preserved |
| `TestExtensionSearchAndConnectionReplay` | `GET /api/bookmarks/?q=...&limit=5` omnibox search with `next` carrying `q`/`limit`/`offset`, and the `GET /api/bookmarks/?limit=1` connection test |
| `TestExtensionDeleteReplay` | `DELETE /api/bookmarks/{id}/` → `204` with an empty body, then `404` for detail and for a repeat delete |

Field parity (`is_archived`, `date_added`, `date_modified` on create/PUT/PATCH
with validation and persistence) and `GET /api/tags/` `limit`/`offset`
pagination are covered by the `internal/api/linkding` tests that
`make test-compat` runs.

## Explicit non-requirements for Phase 1/3

The current official extension does not call archive, unarchive, PUT, PATCH,
asset or bundle endpoints. They remain part of the Linkding-compatible API
scope from `SPEC.md`, but they are not extension blockers. Public shared
bookmark routes are also not part of token-authenticated extension behavior.

Also explicitly deferred, and visible in the API as documented no-ops:

* **metadata fetching** — `check` never performs a network request, so
  `metadata.title`/`description`/`preview_image` are `null`, `auto_tags` is
  always `[]` and `ignore_cache` is accepted and ignored. Real fetching is
  P2-006 and carries a mandatory security review;
* **SingleFile HTML snapshots, assets and bundles** — no snapshot is
  scheduled, so `disable_html_snapshot` (like `disable_scraping`) is accepted
  and ignored, and no asset/bundle route exists;
* **browser automation** — replay plus a manual Firefox run is the E2E gate;
  Chrome stays explicitly unverified when the environment has no tooling for
  it.

## Open compatibility risks

1. Upstream can add extension calls after this revision. Pin a source revision
   in compatibility documentation and rerun request tracing against each
   extension release.
2. `check` performs metadata loading in Linkding. Lynk should return the same
   shape while making metadata retrieval bounded/asynchronous where possible;
   the extension tolerates `null` metadata but not a malformed JSON shape.
3. The extension's current save implementation relies on Linkding's
   create-is-update-on-duplicate behavior. Implementing strict duplicate
   rejection would break the official extension.
4. CORS, HTTPS and reverse-proxy handling must allow the browser extension to
   send `Authorization` and JSON requests. This is a deployment and
   integration test requirement, not an API schema change.

## References

* <https://github.com/sissbruecker/linkding>
* <https://github.com/sissbruecker/linkding-extension>
* <https://linkding.link/api/>
