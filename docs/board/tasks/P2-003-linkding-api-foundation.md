# P2-003 — Fundament API Linkdinga

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P0` |
| Owner | Hy4 Flash Free |
| Faza | Phase 3 przeciągnięta do przodu decyzją PO |
| Zależności | P1-003, P2-001, P2-002 |

## Cel

Uruchomić pierwszy, kompatybilny kontrakt HTTP Linkdinga na bazie gotowej
domeny bookmarków i auth. Zadanie obejmuje DTO, router oraz minimalny zestaw
endpointów potrzebny do połączenia klienta: profile, tags, bookmark check i
podstawowe list/create/get/delete.

## Zakres

* `internal/api/linkding`: DTO, błędy JSON, router i handlery;
* canonical trailing-slash routes;
* `GET /api/user/profile/`;
* `GET /api/tags/`;
* `GET /api/bookmarks/check/`;
* `GET/POST /api/bookmarks/`;
* `GET/DELETE /api/bookmarks/{id}/`;
* auth middleware z P2-002, owner scoping i statusy `401/404/200/201/204`;
* realne testy `httptest` na SQLite i uruchomienie `make test-compat`.

## Poza zakresem

Paginacja/search FTS, update/archive, metadata fetching, HTML snapshots,
public sharing, UI i browser-extension E2E — będą osobnymi zadaniami, jeśli
nie zmieszczą się bez ryzyka w tym slice.

## Kryteria akceptacji

- [x] DTO nie ujawniają pól wewnętrznych i mają stabilny JSON.
- [x] Wszystkie trasy wymagają trailing slash i zachowują `/health`.
- [x] Profile, tags, check i CRUD bookmarków działają z prawidłowym tokenem.
- [x] Cross-user access zwraca `404`, błędne auth `401`.
- [x] Create zwraca `201`, delete puste `204`, błędy mają JSON.
- [x] Testy kontraktowe używają prawdziwego SQLite i `make test-compat` nie jest
  już no-opem.
- [x] `go test -race ./...`, `go vet ./...`, `make check` i build przechodzą.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `READY` | PO | Zadanie utworzone po zamknięciu auth foundation |
| 2026-11-25 | `IN_PROGRESS` | Hy4 Flash Free | Przydzielono implementację w izolowanym worktree |
| 2026-11-25 | `DONE` | Hy4 Flash Free | DTO/errors/router w `internal/api/linkding`, auth wiring w `internal/server`, profile/tags/check oraz list/create/get/delete, testy `httptest` na realnym SQLite; pełna walidacja (gofmt, vet, test, race, `make test-compat`, `make check`, mod tidy/verify, build) |

## Decyzje

Pełna zgodność pagination/search oraz pozostałe endpointy zostaną rozbite na
następne zadania po ocenie rozmiaru tego slice.

## Raport zakresu i pozostałe luki

Zaimplementowano wyłącznie fundament: stałe DTO (bez pól wewnętrznych),
field-keyed błędy walidacji, kanoniczne trasy z trailing slash (bez redirectów),
middleware `Token`/`Bearer` z P2-002 oraz endpointy `GET /api/user/profile/`,
`GET /api/tags/`, `GET /api/bookmarks/check/`, `GET/POST /api/bookmarks/` i
`GET/DELETE /api/bookmarks/{id}/` z owner scopingiem. Listy zwracają envelope
`{count,next,previous,results}` z `next`/`previous` = `null`.

Odroczone (celowo poza tym slice): interpretacja `q`, `limit`, `offset`,
`modified_since`, `added_since`, filtrów i sortowania; `/archived/`, `PUT`,
`PATCH`, `archive`/`unarchive`, metadata scraping i `auto_tags` (zawsze `[]`),
HTML snapshots, `web_archive_snapshot_url`/`favicon_url`/`preview_image_url`
(zawsze `null`), `date_added`/`date_modified` z requestu, `is_archived` przy
create, public sharing, assets/bundles oraz E2E z rozszerzeniem.
