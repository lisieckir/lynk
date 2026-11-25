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

- [x] CLI tworzy/używa użytkownika, migruje bazę i nie uruchamia HTTP listenera.
- [x] Surowy token jest drukowany raz; nie trafia do bazy, logów ani błędów.
- [x] Token ma dokładnie 40 małych znaków hex i digest SHA-256 w bazie.
- [x] `Token`/`Bearer` są case-insensitive; niepoprawne tokeny dają
  `ErrUnauthorized`.
- [x] Revoked/unknown token nie uwierzytelnia użytkownika.
- [x] Middleware wstawia właściciela do kontekstu i nie loguje sekretów.
- [x] `gofmt`, `go vet`, `go test`, `go test -race`, `go mod tidy -diff`
  i `go mod verify` przechodzą.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `READY` | OpenCode | Zadanie utworzone po decyzji PO |
| 2026-11-25 | `IN_PROGRESS` | Iceman Free | Implementacja w izolowanym worktree |
| 2026-11-25 | `IN_PROGRESS` | PO | Review funkcjonalny APPROVE; 6 nowych lint findings wymagają poprawy |

## Blockery

Pozostało 6 zgłoszeń `errcheck` w CLI/auth. Endpointy Linkdinga pozostają
zakresem kolejnego zadania.

## Raport końcowy

Zmienione pliki: `internal/auth/*`, `cmd/lynk/main.go` i testy CLI.
Weryfikacja: build, test, race, vet, tidy/verify PASS; review niezależny APPROVE.
Follow-up: utworzyć zadanie na DTO/router/endpointy Linkdinga.
