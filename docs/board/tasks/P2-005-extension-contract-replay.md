# P2-005 — Extension contract replay i parity pól

| Pole | Wartość |
|---|---|
| Status | `IN_PROGRESS` |
| Priorytet | `P0` |
| Owner | Nemotron Lightning Free |
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

## Kryteria akceptacji

- [ ] Replay pokrywa wszystkie krytyczne żądania przypiętej wersji rozszerzenia.
- [ ] Check ma kompletny stabilny shape, a `ignore_cache` jest akceptowane.
- [ ] Daty i `is_archived` są walidowane oraz zapisywalne zgodnie z kontraktem.
- [ ] Tags respektuje `limit`/`offset` i zwraca pagination envelope.
- [ ] `make check`, `make test-compat`, `make test-integration`, race i build
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
