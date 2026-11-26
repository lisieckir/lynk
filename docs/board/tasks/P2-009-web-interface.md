# P2-009 — Web interface MVP

| Pole | Wartość |
|---|---|
| Status | `BLOCKED` |
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

- [ ] Niezalogowany użytkownik nie może czytać ani mutować bookmarków.
- [ ] Login tworzy bezpieczną, wygasającą sesję bez zapisywania surowego tokenu.
- [ ] Mutacje odrzucają brakujący/niepoprawny CSRF token.
- [ ] Lista, search, tags, archive, create, edit i delete działają przez UI.
- [ ] Wszystkie rekordy są owner-scoped; output jest escapowany przez templates.
- [ ] Security headers, request limits i cookie policy są testowane.
- [ ] `make check`, test/race, compatibility, integration i build przechodzą.
- [ ] Niezależny security review APPROVE i osobny PO_REVIEW.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Iceman Free | Przydzielono implementację web interface MVP |
| 2026-11-26 | `BLOCKED` | Big Pickle | Security review REQUEST CHANGES: lint blocker, login DoS/rate limit, logout CSRF, test gaps and duplicate-create data loss |
| 2026-11-26 | `BLOCKED` | Big Pickle | Follow-up still incomplete: exact pinned linter unavailable, configuration and exhaustive security/flow tests missing |
| 2026-11-26 | `BLOCKED` | PO | `80197f1` passes exact pinned linter, but env parsing and exhaustive security/flow tests remain incomplete |

## Blockery

Nie integrować `80c03e4` przed naprawą findings, pełną macierzą z pinned
linterem i ponownym niezależnym security review.
