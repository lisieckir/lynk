# P1-003 — SQLite, pragmas i migracje

| Pole | Wartość |
|---|---|
| Status | `DONE` |
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

- [x] Nowa baza inicjalizuje się automatycznie.
- [x] Istniejąca baza nie jest nadpisywana.
- [x] Migracje są wykonywane w kolejności i są odporne na restart.
- [x] Test potwierdza `PRAGMA foreign_keys = ON`.
- [x] Test potwierdza WAL i `busy_timeout`.
- [x] Test `CREATE VIRTUAL TABLE ... USING fts5` przechodzi.
- [x] Backup uwzględnia WAL i przechodzi `PRAGMA integrity_check`.

## Weryfikacja

Testy SQLite na Linux AMD64 oraz test ponownego uruchomienia na tej samej
bazie.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Oczekuje na P1-001 |
| 2026-11-25 | `IN_PROGRESS` | Big Pickle | Rozpoczęto walidację drivera, SQLite foundation i migracji |
| 2026-11-25 | `DONE` | Flint 40B Free | Review zaakceptował poprawki; 16 testów i race detector przechodzą |

## Blockery

Brak. Driver `modernc.org/sqlite v1.63.0` zweryfikowany na Linux AMD64.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| 2026-11-25 | Online Backup API czy `VACUUM INTO` jako główna ścieżka | Wybrano SQLite Online Backup API; WAL/integrity testy przechodzą | Flint 40B Free |
| 2026-11-25 | FTS5 synchronizacja tagów | Dodano migrację `0002_bookmark_tag_fts_triggers.sql` i testy regresyjne | Flint 40B Free |
| 2026-11-25 | Prawa nowych plików bazy | `0600`; istniejące prawa są zachowywane | Flint 40B Free |
