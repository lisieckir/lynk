# P2-001 — Domena zakładek, repository i service

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P1` |
| Owner | GPT-6 Luna |
| Faza | Phase 2 |
| Zależności | P1-003 |

## Cel

Zbudować wewnętrzną domenę zakładek na bazie istniejącego SQLite foundation,
bez implementowania jeszcze Linkding API ani UI.

## Zakres

* typy domenowe bookmark/tag/user;
* repository oparte o `database/sql`;
* service dla create/update/delete/list/retrieve;
* duplicate POST po `user_id + url_normalized` jako merge/update;
* PUT/PATCH collision jako błąd domenowy;
* archive/unarchive;
* tag replacement i synchronizacja z FTS5;
* transakcje i context cancellation.

## Poza zakresem

* HTTP API i DTO Linkdinga;
* token authentication;
* HTML/UI;
* metadata fetching;
* wieloużytkownikowy panel administracyjny.

## Kryteria akceptacji

- [x] CRUD działa w testach na prawdziwym SQLite.
- [x] Duplicate create ma zachowanie zgodne z `docs/api-compatibility.md`.
- [x] Tag replacement jest atomowy i aktualizuje FTS5.
- [x] Archive/unarchive aktualizuje `date_modified`.
- [x] Operacje są ograniczone do właściciela.
- [x] Rollback nie zostawia częściowych relacji.
- [x] `go test -race ./...` przechodzi.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `IN_PROGRESS` | GPT-6 Luna | Rozpoczęto implementację domeny |
| 2026-11-25 | `REVIEW` | OpenCode | Implementacja gotowa do niezależnego review |
| 2026-11-25 | `PAUSED` | OpenCode | Wstrzymano przed poprawką portu `:0` i malformed percent escapes |
| 2026-11-25 | `DONE` | GPT-6 Luna | Review końcowy zaakceptował domenę i testy |

## Blockery

Brak. P1-007 pozostaje niezależnym blockerem środowiskowym.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| 2026-11-25 | Model service/repository i normalizacja URL | Service/repository bez zbędnych interfejsów; normalizacja zweryfikowana względem upstream i pokryta testami | Review końcowy |
