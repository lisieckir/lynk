# P1-007 — Walidacja i pomiary Phase 1

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P1` |
| Owner | GPT-6 Luna |
| Faza | Phase 1 |
| Zależności | P1-001, P1-002, P1-003, P1-004, P1-005, P1-006 |

## Cel

Potwierdzić, że fundament Phase 1 jest uruchamialny, powtarzalny i gotowy do
Phase 2.

## Kryteria akceptacji

- [x] `make check` przechodzi bez waivera lintera (`golangci-lint v2.15.0`, 0 issue'ów).
- [x] `make test-race` przechodzi.
- [x] `make LINT_TOOL=none docker-release` przechodzi; bez-waiverowy `make check` nadal wymaga lintera.
- [x] Healthcheck działa w Docker Compose.
- [x] Migracje działają na świeżej i istniejącej bazie.
- [x] Zmierzono rozmiar binarki, obrazu i bazowe zużycie pamięci kontenera.
- [x] Wyniki zapisano w dokumentacji projektu.
- [x] PO zaakceptował deliverable Phase 1.

## Weryfikacja

Pełny przebieg Makefile na czystym środowisku oraz ręczna kontrola danych po
restartach.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Zadanie końcowe Phase 1 |
| 2026-11-25 | `IN_PROGRESS` | GPT-6 Luna | Rozpoczęto pełną walidację Phase 1 i pomiary |
| 2026-11-25 | `BLOCKED` | GPT-6 Luna | Go/SQLite PASS; Docker/Compose i pełny lint zablokowane przez brak narzędzi |
| 2026-11-25 | `IN_PROGRESS` | OpenCode | Docker/Compose dostępne; wznowiono walidację kontenera |
| 2026-11-25 | `BLOCKED` | OpenCode | Build, health, restart, recreate i named-volume persistence PASS; brak `golangci-lint` blokuje bez-waiverowy sign-off |
| 2026-11-25 | `BLOCKED` | Big Pickle | `golangci-lint v2.15.0` uruchomiony poza repozytorium; `make check` bez waivera FAIL na 14 issue'ach lintu, `make test` i `make test-race` PASS |
| 2026-11-25 | `DONE` | PO | Wszystkie 14 issue'ów naprawione; niezależny review APPROVE; `make check`, race i Docker validation PASS |

## Blockery

Brak blockerów. Docker/Compose, `golangci-lint`, testy, race, vet i build
zostały zwalidowane.

Zweryfikowane 2026-11-25 przez Big Pickle:

- `golangci-lint v2.15.0` pobrany z oficjalnego release'u GitHub, suma SHA-256
  zweryfikowana z `golangci-lint-2.15.0-checksums.txt`; plik znajduje się poza
  repozytorium w `/tmp/opencode/golangci-lint-v2.15.0/`, `git status` pozostał
  czysty.
- `make check LINT_TOOL=/tmp/opencode/golangci-lint-v2.15.0/golangci-lint-2.15.0-linux-amd64/golangci-lint`
  wykonuje `fmt` i `vet`, następnie kończy się `FAIL` w `lint` (exit 2).
- `make test` i `make test-race` przechodzą.

Zgłoszenia do poprawy: `cmd/lynk/main_test.go:166`,
`internal/bookmark/bookmark.go:102,159,498`, `internal/database/backup.go:43,88`,
`internal/database/backup_test.go:42`, `internal/database/database.go:212`,
`internal/database/database_test.go:243`,
`internal/server/server_test.go:60,280,296,299`,
`internal/shutdown/shutdown_test.go:118`.

Następny krok: poprawić zgłoszone błędy (nieobsłużone `Close` oraz `nil`
`Context` w testach), a następnie ponownie uruchomić `make check` z tym samym
linterem. `LINT_TOOL=none` nie może być użyty jako potwierdzenie tego kryterium.
