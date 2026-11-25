# P2-002 — Uwierzytelnianie tokenem i bootstrap CLI

| Pole | Wartość |
|---|---|
| Status | `IN_PROGRESS` |
| Priorytet | `P0` |
| Owner | Iceman Free |
| Faza | Phase 3 (API Linkdinga) przeciągnięta do przodu decyzją PO |
| Zależności | P1-003, P2-001 |

## Cel

Zbudować fundament uwierzytelniania API tokenem oraz CLI do utworzenia
pierwszego użytkownika i tokenu. Endpointy HTTP Linkdinga pozostają poza tym
zadaniem.

## Decyzje PO

* ID `P2-002` pozostaje kolejnym zadaniem boardu, mimo że zakres pochodzi z
  Phase 3.
* Bootstrap odbywa się przez `lynk token create`; token surowy jest drukowany
  dokładnie raz.
* Token ma 40 małych znaków hex z `crypto/rand`; w bazie jest tylko SHA-256
  digest i krótki prefix.
* Parser akceptuje case-insensitive `Token` i `Bearer`.
* `lynk token revoke --prefix PREFIX` odwołuje token.

## Zakres

* `internal/auth`: generowanie, digest, walidacja, lookup i revocation tokenów;
* middleware i `user_id` w kontekście żądania;
* `lynk token create` oraz `lynk token revoke`;
* testy na prawdziwym SQLite, testy middleware i redakcji sekretów;
* brak nowych zależności produkcyjnych.

## Poza zakresem

* endpointy, DTO i router API Linkdinga;
* paginacja, wyszukiwanie i metadata;
* sesje, hasła, UI, rate limiting i CORS.

## Kryteria akceptacji

- [ ] CLI tworzy/używa użytkownika, migruje bazę i nie uruchamia HTTP listenera.
- [ ] Surowy token jest drukowany raz; nie trafia do bazy, logów ani błędów.
- [ ] Token ma dokładnie 40 małych znaków hex i digest SHA-256 w bazie.
- [ ] `Token`/`Bearer` są case-insensitive; niepoprawne tokeny dają
  `ErrUnauthorized`.
- [ ] Revoked/unknown token nie uwierzytelnia użytkownika.
- [ ] Middleware wstawia właściciela do kontekstu i nie loguje sekretów.
- [ ] `gofmt`, `go vet`, `go test`, `go test -race`, `go mod tidy -diff`
  i `go mod verify` przechodzą.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `READY` | OpenCode | Zadanie utworzone po decyzji PO |
| 2026-11-25 | `IN_PROGRESS` | Iceman Free | Implementacja w izolowanym worktree |

## Blockery

P1-007 ma niezależny blocker: 14 zgłoszeń `golangci-lint`. Nie blokuje pracy
nad auth, ale integracja wymaga osobnego review.

## Raport końcowy

Wypełnić przed `DONE`: zmienione pliki, testy, ograniczenia i follow-upy.
