# P2-004 — Search, pagination i mutacje bookmarków Linkdinga

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P0` |
| Owner | Tidepool Alpha Free |
| Faza | Phase 3 API |
| Zależności | P2-003 |

## Cel

Domknąć krytyczny kontrakt bookmarków używany przez oficjalne rozszerzenie i
API Linkdinga: prawdziwy search FTS, pagination oraz pełne mutacje/archive.

## Zakres

* `q`, `limit`, `offset` oraz envelope `{count,next,previous,results}`;
* `/api/bookmarks/archived/`;
* `PUT` i `PATCH /api/bookmarks/{id}/`;
* `POST .../archive/` i `.../unarchive/`;
* bezpieczne zapytania FTS5 obejmujące URL, tytuł, opis/notatki i tagi;
* owner scoping, collision/validation mapping i puste `204`;
* realne compatibility tests zgodne z `docs/api-compatibility.md`.

## Poza zakresem

Metadata fetching, HTML snapshots, assets/bundles, sharing, rate limiting i
browser-extension E2E.

## Kryteria akceptacji

- [x] `q` działa po FTS5 i nie pozwala na raw SQL/FTS injection.
- [x] `limit`/`offset` są walidowane i odwzorowane w `count,next,previous,results`.
- [x] Archived list i archive/unarchive działają z owner scoping.
- [x] PUT wymaga URL; PATCH jest częściowy; collision zwraca `400`.
- [x] Mutacje zwracają kontraktowe `200`/`204`, a nieudane ownership `404`.
- [x] Compatibility tests pokrywają search, pagination, update i archive.
- [x] `make check`, `make test-compat`, `go test -race ./...`, build i verify
  przechodzą.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `IN_PROGRESS` | Tidepool Alpha Free | Przydzielono implementację w izolowanym worktree |
| 2026-11-25 | `DONE` | Tidepool Alpha Free | Search FTS5 (`internal/bookmark`), paging `{count,next,previous,results}` z limit/offset (domyślny 100), `/api/bookmarks/archived/`, PUT/PATCH z kontraktem błędów `400`, `POST .../archive/` + `.../unarchive/` (`204`). Dodaano compat testy w `internal/api/linkding/compatibility_test.go` (real SQLite + httptest): search pól FTS, wtryskowe `q`, paging z next/previous, PUT/PATCH, archiwizacja. Weryfikowane: `go test ./...`, `make test-compat` |
| 2026-11-25 | `DONE` | Tidepool Alpha Free | Prefiks ostatniego tokenu `q` (omnibox dopisujący częściowe słowa), testy jednostkowe `internal/bookmark/search_test.go`, udokumentowane różnice w `docs/api-compatibility.md`. Pełna bramka: `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, `make test-compat`, `make check` (golangci-lint 2.15.0, 0 issues), `make build`, `go mod tidy`/`go mod verify` |

## Ryzyka i decyzje

Linkding używa request-derived `next`/`previous`; implementacja może zwracać
`null` na końcach stron, ale musi zachować klucze i prawidłowy limit.

`q` jest bezpieczne (każdy token we frazie, prefiks tylko dopisywany przez
serwer), więc wtręty FTS5/SQL pozostają inertne. Prefiks ostatniego tokenu
zastępuje pełny substring Linkdinga: częściowe słowa z omniboxa działają,
ale fragment w środku słowa (`lph`) nie trafia w `alpha` — udokumentowano
w `docs/api-compatibility.md` i pokryto testami.

## Raport zakresu i pozostałe luki

Zaimplementowano: `q`/`limit`/`offset` w `GET /api/bookmarks/` i nowym
`GET /api/bookmarks/archived/` (envelope `{count,next,previous,results}`,
`count` = pełna liczba dopasowań, `next`/`previous` request-derived i `null`
na końcach), wyszukiwanie FTS5 po URL/tytule/opisie/notatkach/tagach w
domenowej metodzie `bookmark.Service.List` (bez SQL w handlerach), `PUT`
(wymaga `url`) i `PATCH /api/bookmarks/{id}/` z mapowaniem konfliktu URL na
`400`, `POST .../archive/` i `.../unarchive/` z pustym `204`, owner scoping
(`404` dla cudzych/brakujących wierszy) i kontraktowe statusy.

Odroczone (poza tym slice): interpretacja `limit`/`offset` dla
`GET /api/tags/` (zwraca pełną listę), filtry `modified_since`,
`added_since`, `sort`, `unread`, `shared`, `bundle`, prefetch metadata,
zasoby/assets, bundlery, public sharing i UI/E2E rozszerzenia.
