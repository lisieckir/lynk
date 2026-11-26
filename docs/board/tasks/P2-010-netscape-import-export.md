# P2-010 — Netscape HTML import/export

| Pole | Wartość |
|---|---|
| Status | `BLOCKED` |
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

- [ ] Import accepts common Netscape HTML and preserves URL/title/notes/tags.
- [ ] Export is valid Netscape HTML and round-trips without XSS or data loss.
- [ ] User selection is explicit and owner-scoped.
- [ ] Duplicate and malformed input behavior is deterministic and documented.
- [ ] Limits prevent unbounded memory/file processing.
- [ ] CLI errors do not leak secrets or overwrite source/destination silently.
- [ ] `make check`, test/race, build and compatibility pass.
- [ ] Independent review APPROVE and PO_REVIEW.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Hy4 Flash Free | Przydzielono import/export Phase 7 |
| 2026-11-26 | `REVIEW` | Hy4 Flash Free | `043cba0` implementuje parser, bounded CLI, atomic import i round-trip; niezależny review w toku |
| 2026-11-26 | `BLOCKED` | Big Pickle | Review REQUEST CHANGES: symlinked-parent DB overwrite (HIGH) and cumulative field-cap bypass (MEDIUM) |

## Blockery

Nie integrować `043cba0` przed naprawą destination canonicalization, cumulative
field limits i regresyjnymi testami bezpieczeństwa oraz ponownym review.
