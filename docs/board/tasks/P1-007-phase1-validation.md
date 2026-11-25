# P1-007 — Walidacja i pomiary Phase 1

| Pole | Wartość |
|---|---|
| Status | `BLOCKED` |
| Priorytet | `P1` |
| Owner | GPT-6 Luna |
| Faza | Phase 1 |
| Zależności | P1-001, P1-002, P1-003, P1-004, P1-005, P1-006 |

## Cel

Potwierdzić, że fundament Phase 1 jest uruchamialny, powtarzalny i gotowy do
Phase 2.

## Kryteria akceptacji

- [ ] `make check` przechodzi bez waivera lintera.
- [x] `make test-race` przechodzi.
- [x] `make LINT_TOOL=none docker-release` przechodzi; bez-waiverowy `make check` nadal wymaga lintera.
- [x] Healthcheck działa w Docker Compose.
- [x] Migracje działają na świeżej i istniejącej bazie.
- [x] Zmierzono rozmiar binarki, obrazu i bazowe zużycie pamięci kontenera.
- [x] Wyniki zapisano w dokumentacji projektu.
- [ ] PO zaakceptował deliverable Phase 1.

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

## Blockery

Docker Engine i Compose zostały zwalidowane. Jedyny pozostały blocker to brak
`golangci-lint`, przez co `make check` bez waivera nie może przejść.
