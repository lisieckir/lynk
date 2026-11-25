# P1-001 — Bootstrap Go module i narzędzi

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P1` |
| Owner | Big Pickle |
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
* przygotować minimalny punkt wejścia do późniejszego użycia przez Makefile.

### Poza zakresem

* API Linkdinga;
* model domenowy zakładek;
* UI;
* Dockerfile i Compose.

## Kryteria akceptacji

- [x] `go test ./...` działa na czystym checkoutcie.
- [x] `go mod tidy`, `go mod verify` nie zgłaszają problemów.
- [x] Wersje Go i zależności są jawnie udokumentowane
      (`docs/development.md`, `docs/architecture.md`).
- [x] Wybrana wersja `modernc.org/sqlite` jest przypięta i uzasadniona;
      P1-003 zweryfikuje jej FTS5/WAL/backup.
- [x] Nie dodano zależności bez uzasadnienia.
- [x] Nie ma jeszcze funkcji aplikacyjnych poza uruchamianiem fundamentu.

## Weryfikacja

| Komenda | Wynik (2026-11-25, Go `1.27.4`) |
|---|---|
| `gofmt -l .` | brak plików wymagających formatowania |
| `go build ./...` | sukces, błędów brak |
| `go test ./...` | `ok github.com/lisieckir/lynk/cmd/lynk` (`TestBootstrapEntrypoint`) |
| `go vet ./...` | sukces, błędów brak |
| `go mod tidy -diff` | brak różnic |
| `go mod verify` | `all modules verified` |

`go build ./cmd/lynk` jest objęty przez `go build ./...`. Testy domenowe,
SQLite i HTTP są celowo odroczone do P1-003/P1-004; P1-001 zawiera tylko
smoke-test potwierdzający kompilację i czysty powrót entrypointu.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `READY` | — | Zadanie przygotowane |
| 2026-11-25 | `IN_PROGRESS` | Big Pickle | Agent rozpoczął pracę w izolowanym worktree |
| 2026-11-25 | `REVIEW` | OpenCode | Implementacja gotowa do niezależnego code review |
| 2026-11-25 | `REVIEW` | Big Pickle | Poprawki po code review: dokumentacja wersji jako prowizorycznych, README, ignorowanie `.env`, smoke-test i ponowna weryfikacja |
| 2026-11-25 | `DONE` | Flint 40B Free | P1-003 potwierdziło pin SQLite; implementacja i pełna weryfikacja zaakceptowane |

## Blockery

Implementacja zaakceptowana przez code review. Pin Go/SQLite został
potwierdzony przez P1-003 na Linux AMD64.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| 2026-11-25 | Go `1.27.4` i `modernc.org/sqlite` `v1.63.0` jako piny P1-001 | P1-003 potwierdziło FTS5/WAL/backup na Linux AMD64; cross-platform pozostaje release follow-up | Flint 40B Free |
| 2026-11-25 | Dodanie dyrektywy `toolchain` w `go.mod` | Nie dodano: `go 1.27.4` wyznacza minimalną wersję, a dyrektywa mogłaby wymusić przełączenie toolchainu; patrz `docs/development.md` | — |
| 2026-11-25 | Niezależny code review po poprawkach | APPROVE warunkowo; brak blockerów technicznych, pin oczekuje na P1-003 | Flint 40B Free |
