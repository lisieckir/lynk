# P1-003 — SQLite, pragmas i migracje

| Pole | Wartość |
|---|---|
| Status | `IN_PROGRESS` |
| Priorytet | `P0` |
| Owner | Big Pickle |
| Faza | Phase 1 |
| Zależności | P1-001 |

## Cel

Uruchomić bezpieczną warstwę SQLite z migracjami i potwierdzić założenia
sterownika `modernc.org/sqlite`.

## Zakres

* `database/sql` i ograniczona pula połączeń;
* `foreign_keys`, WAL i `busy_timeout`;
* tabela wersji migracji;
* pierwsza migracja schematu z `docs/database-schema.md`;
* smoke test FTS5;
* test transakcji i rollbacku;
* proof-of-concept spójnego backupu.

## Kryteria akceptacji

- [ ] Nowa baza inicjalizuje się automatycznie.
- [ ] Istniejąca baza nie jest nadpisywana.
- [ ] Migracje są wykonywane w kolejności i są odporne na restart.
- [ ] Test potwierdza `PRAGMA foreign_keys = ON`.
- [ ] Test potwierdza WAL i `busy_timeout`.
- [ ] Test `CREATE VIRTUAL TABLE ... USING fts5` przechodzi.
- [ ] Backup uwzględnia WAL i przechodzi `PRAGMA integrity_check`.

## Weryfikacja

Testy SQLite na Linux AMD64 oraz test ponownego uruchomienia na tej samej
bazie.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Oczekuje na P1-001 |
| 2026-11-25 | `IN_PROGRESS` | Big Pickle | Rozpoczęto walidację drivera, SQLite foundation i migracji |

## Blockery

Brak.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| — | Online Backup API czy `VACUUM INTO` jako główna ścieżka | Otwarte | — |
