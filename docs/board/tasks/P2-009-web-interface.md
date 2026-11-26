# P2-009 — Web interface MVP

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P0` |
| Owner | Iceman Free |
| Faza | Phase 5 |
| Zależności | P2-008 |

## Cel

Zrealizować brakujący deliverable Definition of Done: użyteczny,
server-rendered web interface bez Node.js, z listą, wyszukiwaniem, tagami,
tworzeniem, edycją, archiwizacją i usuwaniem bookmarków.

## Zakres

* `html/template`, `embed`, HTMX i plain CSS/static assets;
* bezpieczna web-session oparta o istniejący API token bootstrapu, z
  `HttpOnly`, `Secure` w production, `SameSite` i rotacją/wygaśnięciem;
* CSRF token dla mutacji formularzy, security headers i bounded request bodies;
* ekran logowania tokenem, lista/pagination/search/filter archived, create/edit,
  delete/archive/unarchive i tag display;
* owner scoping przez istniejący `bookmark.Service`, bez bezpośrednich
  niezabezpieczonych SQL query w handlerach;
* responsive layout i jasne błędy; no-JS fallback dla podstawowych operacji;
* testy handler/session/CSRF i HTTP integration; brak publicznych sekretów.

## Poza zakresem

Public sharing, multi-user administration, bulk operations, reader mode,
SingleFile/assets/bundles i nowy frontend build pipeline.

## Kryteria akceptacji

- [x] Niezalogowany użytkownik nie może czytać ani mutować bookmarków.
- [x] Login tworzy bezpieczną, wygasającą sesję bez zapisywania surowego tokenu.
- [x] Mutacje odrzucają brakujący/niepoprawny CSRF token.
- [x] Lista, search, tags, archive, create, edit i delete działają przez UI.
- [x] Wszystkie rekordy są owner-scoped; output jest escapowany przez templates.
- [x] Security headers, request limits i cookie policy są testowane.
- [x] `make check`, test/race, compatibility, integration i build przechodzą.
- [x] Niezależny security review APPROVE i osobny PO_REVIEW.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Iceman Free | Przydzielono implementację web interface MVP |
| 2026-11-26 | `BLOCKED` | Big Pickle | Security review REQUEST CHANGES: lint blocker, login DoS/rate limit, logout CSRF, test gaps and duplicate-create data loss |
| 2026-11-26 | `BLOCKED` | Big Pickle | Follow-up still incomplete: exact pinned linter unavailable, configuration and exhaustive security/flow tests missing |
| 2026-11-26 | `BLOCKED` | PO | `80197f1` passes exact pinned linter, but env parsing and exhaustive security/flow tests remain incomplete |
| 2026-11-26 | `BLOCKED` | PO | `0c27d07` adds env parsing and representative tests; final security review pending because exhaustive matrix remains partial |
| 2026-11-26 | `DONE` | PO | Security review `APPROVE`, full pinned matrix PASS, production cookie test and env docs added |

## Blockery

Brak blockerów. P2-009 jest zintegrowane po security `APPROVE` i PO_REVIEW.
