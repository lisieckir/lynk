# P2-010 — Netscape HTML import/export

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P0` |
| Owner | Hy4 Flash Free |
| Faza | Phase 7 |
| Zależności | P2-009 |

## Cel

Dostarczyć migration-ready Netscape HTML import/export bez utraty danych,
niebezpiecznego HTML ani niejawnego wyboru właściciela.

## Zakres

* stdlib-only parser Netscape bookmark HTML z title, URL, tags/folder,
  description/notes i archived/unread mapping zgodnie z dokumentacją;
* deterministyczny, poprawnie escapowany Netscape export;
* jawny CLI command z wymaganym username/user id oraz ścieżką pliku/stdin/stdout;
* duplicate URL handling zgodny z zaakceptowanym service/API semantics;
* bounded input size, malformed input errors, atomic import transaction and
  summary of created/merged/skipped/failed records;
* no overwrite of source files; export uses explicit destination policy;
* tests for entities, Unicode, malformed HTML, duplicates, owner isolation,
  HTML escaping, limits and round-trip.

## Poza zakresem

Public sharing, browser import UI, JSON/CSV formats, remote URLs and background
network fetches.

## Kryteria akceptacji

- [x] Import accepts common Netscape HTML and preserves URL/title/notes/tags.
- [x] Export is valid Netscape HTML and round-trips without XSS or data loss.
- [x] User selection is explicit and owner-scoped.
- [x] Duplicate and malformed input behavior is deterministic and documented.
- [x] Limits prevent unbounded memory/file processing.
- [x] CLI errors do not leak secrets or overwrite source/destination silently.
- [x] `make check`, test/race, build and compatibility pass.
- [x] Independent review APPROVE and PO_REVIEW.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Hy4 Flash Free | Przydzielono import/export Phase 7 |
| 2026-11-26 | `REVIEW` | Hy4 Flash Free | `043cba0` implementuje parser, bounded CLI, atomic import i round-trip; niezależny review w toku |
| 2026-11-26 | `BLOCKED` | Big Pickle | Review REQUEST CHANGES: symlinked-parent DB overwrite (HIGH) and cumulative field-cap bypass (MEDIUM) |
| 2026-11-26 | `BLOCKED` | Hy4 Flash Free | `9dcd3da` naprawia findings; pełna macierz PASS, ponowny niezależny review w toku |
| 2026-11-26 | `DONE` | PO | Re-review `APPROVE`; symlink/cap/UTF-8 fixes and full matrix PASS; PO_REVIEW PASS |

## Blockery

Brak blockerów. P2-010 jest zintegrowane po niezależnym security `APPROVE`.
