# P2-005 — Extension contract replay i parity pól

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P0` |
| Owner | MiMo-V2.6-Flash Free |
| Faza | Phase 3→4 bridge |
| Zależności | P2-004 |

## Cel

Zabezpieczyć kontrakt oficjalnego rozszerzenia testem replay oraz domknąć
parity pól zapisywalnych i paginacji tagów przed prawdziwym E2E.

## Zakres

* `tests/integration/` z request replay dla profilu, tags, check, detail,
  save/merge, search, connection test i delete;
* `make test-integration` uruchamia realne testy na SQLite/httptest;
* `is_archived`, `date_added`, `date_modified` w create/PUT/PATCH;
* `limit`/`offset` na `/api/tags/`;
* dokumentacja `ignore_cache`, `auto_tags: []` i SingleFile jako odroczonego;
* brak sieciowego metadata fetchera — to osobne P2-006.

## Poza zakresem

Metadata fetching (osobne P2-006 z obowiązkowym security review), SingleFile,
zasoby/assets, bundlery, pobieranie metadanych z sieci, UI oraz prawdziwy
przeglądarkowy E2E. Żadnego z tych elementów nie implementowano w tym
zadaniu.

## Kryteria akceptacji

- [x] Replay pokrywa wszystkie krytyczne żądania przypiętej wersji rozszerzenia.
- [x] Check ma kompletny stabilny shape, a `ignore_cache` jest akceptowane.
- [x] Daty i `is_archived` są walidowane oraz zapisywalne zgodnie z kontraktem.
- [x] Tags respektuje `limit`/`offset` i zwraca pagination envelope.
- [x] `make check`, `make test-compat`, `make test-integration`, race i build
  przechodzą.

## Decyzje PO

* Prawdziwe metadata fetching jest wymagane przed release’em, ale ma osobne
  zadanie P2-006 z obowiązkowym security review.
* SingleFile/assets/bundles są odroczone poza MVP.
* Replay + manualny Firefox są pierwszym E2E gate; Chrome pozostaje jawnie
  niezweryfikowany, jeśli środowisko nie dostarczy narzędzi.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | PO | Przygotowane po analizie pozostałych luk API |
| 2026-11-26 | `IN_PROGRESS` | Nemotron Lightning Free | Przydzielono implementację po zamknięciu P2-004 |
| 2026-11-26 | `DONE` | MiMo-V2.6-Flash Free | `tests/integration/extension_replay_test.go`: replay przypiętej wersji rozszerzenia (`d0221d9c5271245217d06e76c49b86a70b0fedb9`) na realnie migrowanym SQLite + `httptest` — `401` bez tokena, profil, `tags/?limit=5000`, `check/?url=…&ignore_cache=true` (i bez flagi), detail, `POST /api/bookmarks/?disable_scraping&disable_html_snapshot` oraz merge zwracający `201` tym samym `id`, search `?q=…&limit=5`, connection test `?limit=1`, `DELETE` → puste `204` i `404`. `make test-integration` uruchamia pakiet |
| 2026-11-26 | `DONE` | MiMo-V2.6-Flash Free | Zapisywalne `is_archived`, `date_added`, `date_modified` w create/PUT/PATCH: walidacja ISO-8601 przed dotknięciem SQL, pole-kluczowe `400` bez zmiany wiersza, merge zgodny z `_merge_bookmark_data`. Paginacja `limit`/`offset` na `/api/tags/` z envelope `{count,next,previous,results}` i `count` jako całkowita liczba tagów. Testy: `internal/api/linkding/writable_fields_test.go`, `tags_pagination_test.go`, `internal/bookmark/lookup_test.go`. Dokumentacja: `docs/api-compatibility.md`, `docs/development.md` |
| 2026-11-26 | `DONE` | MiMo-V2.6-Flash Free | Pełna bramka: `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./...`, `go test -race ./...`, `make test-compat`, `make test-integration`, `make LINT_TOOL=<golangci-lint v2.15.0> check` (0 issues), `go mod tidy -diff`, `go mod verify` — wszystko PASS |

## Ryzyka i decyzje

* Różnica względem przypiętego Linkdinga: usługa `update_bookmark` przy każdym
  `PUT`/`PATCH` nadpisuje `date_modified` zegarem serwera, więc wysłana przez
  klienta wartość jest walidowana, a następnie odrzucana. Lynk zapisuje
  wysłane `date_modified`, a zegar serwera stosuje tylko wtedy, gdy pola nie
  ma. Udokumentowane w `docs/api-compatibility.md` i pokryte testem.
* Merge duplikatu POST kopiuje tylko `title`, `description`, `notes`, `unread`
  i `shared` (jak `_merge_bookmark_data`), więc przesłane `is_archived` i
  `date_added` są walidowane, ale nie zmieniają zapisanego wiersza, a
  `date_modified` idzie „teraz”. Dzięki temu edycja z rozszerzenia nie
  odarchiwizowuje bookmarka i nie kasuje daty utworzenia.
* JSON `null` w polu daty oznacza „niepodane”; pusty string, goła data lub
  zły typ to `400` z kluczem pola, a odrzucona zmiana nie dotyka bazy.
* `GET /api/tags/` bez `limit` zwraca teraz do 100 tagów (domyślny
  `PAGE_SIZE` Linkdinga) zamiast pełnej listy — zgodnie z upstreamem;
  rozszerzenie wysyła `limit=5000` i dostaje cały zestaw na jednej stronie.
* `ignore_cache` i `auto_tags: []` są częścią kontraktu check: flaga jest
  akceptowana i ignorowana, bo Lynk nie trzyma cache metadanych i nie robi
  zapytań sieciowych; `disable_scraping`/`disable_html_snapshot` działają tak
  samo. SingleFile/assets pozostają odroczone.

## Raport zakresu i pozostałe luki

Zaimplementowano: paczkę `tests/integration` z replay czterech przepływów
rozszerzenia (popup init, save/merge, search/connection test, delete) na
realnie migrowanym SQLite przez `httptest`; zapisywalne `is_archived`,
`date_added` i `date_modified` w `POST`, `PUT` i `PATCH` wraz z walidacją
ISO-8601 i persystencją (`internal/bookmark`: `CreateInput`/`UpdateInput`,
insert i update z kolumnami dat); paginację `limit`/`offset` dla
`GET /api/tags/` w `bookmark.Service.ListTags` i handlerze z envelope
`{count,next,previous,results}`; komentarze kontraktowe przy `check`
(`ignore_cache`) i `create` (`disable_scraping`/`disable_html_snapshot`);
odświeżenie `docs/api-compatibility.md` (sekcja pól zapisywalnych, replay,
odroczenia) i `docs/development.md` (układ testów i tabela walidacji P2-005).

Odroczone (poza tym slice): metadata fetching (P2-006 z security review),
SingleFile, assets/bundles, filtry `modified_since`, `added_since`, `sort`,
`unread`, `shared`, `bundle`, public sharing, UI oraz przeglądarkowy E2E
(Chrome jawnie niezweryfikowany). Statusy w `docs/board/STATUS.md` i
`docs/board/PAUSE.md` pozostają do aktualizacji przez PO przy zamykaniu
zadania na tablicy.
