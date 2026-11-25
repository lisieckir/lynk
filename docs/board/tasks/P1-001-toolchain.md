# P1-001 — Bootstrap Go module i narzędzi

| Pole | Wartość |
|---|---|
| Status | `READY` |
| Priorytet | `P1` |
| Owner | — |
| Faza | Phase 1 |
| Zależności | Phase 0 zaakceptowana |

## Cel

Przygotować minimalny, powtarzalny fundament projektu Go bez implementowania
funkcji aplikacyjnych.

## Zakres

### W zakresie

* wybrać i przypiąć wersję Go;
* utworzyć `go.mod`;
* przypiąć `modernc.org/sqlite` po smoke-testach kompatybilności;
* utworzyć minimalny entrypoint `cmd/lynk`;
* dodać `.gitignore` i `.dockerignore`;
* uruchomić podstawowe targety Makefile.

### Poza zakresem

* API Linkdinga;
* model domenowy zakładek;
* UI;
* Dockerfile i Compose.

## Kryteria akceptacji

- [ ] `go test ./...` działa na czystym checkoutcie.
- [ ] `go mod tidy`, `go mod verify` nie zgłaszają problemów.
- [ ] Wersje Go i zależności są jawnie udokumentowane.
- [ ] Nie dodano zależności bez uzasadnienia.
- [ ] Nie ma jeszcze funkcji aplikacyjnych poza uruchamianiem fundamentu.

## Weryfikacja

`make deps`, `make fmt`, `make vet`, `make test`.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `READY` | — | Zadanie przygotowane |

## Blockery

Brak.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| — | Wersja Go i dokładna wersja SQLite drivera | Oczekuje weryfikacji | — |
