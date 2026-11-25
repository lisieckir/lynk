# P2-001 — Domena zakładek, repository i service

| Pole | Wartość |
|---|---|
| Status | `IN_PROGRESS` |
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

- [ ] CRUD działa w testach na prawdziwym SQLite.
- [ ] Duplicate create ma zachowanie zgodne z `docs/api-compatibility.md`.
- [ ] Tag replacement jest atomowy i aktualizuje FTS5.
- [ ] Archive/unarchive aktualizuje `date_modified`.
- [ ] Operacje są ograniczone do właściciela.
- [ ] Rollback nie zostawia częściowych relacji.
- [ ] `go test -race ./...` przechodzi.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `IN_PROGRESS` | GPT-6 Luna | Rozpoczęto implementację domeny |

## Blockery

Brak. P1-007 pozostaje niezależnym blockerem środowiskowym.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| — | Model service/repository i normalizacja URL | Do weryfikacji z API compatibility | — |
