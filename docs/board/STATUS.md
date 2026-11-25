# Lynk Board — status zbiorczy

Ostatnia aktualizacja: 2026-11-25  
Właściciel tablicy: PO / koordynator projektu

## Legenda

`BACKLOG` · `READY` · `IN_PROGRESS` · `BLOCKED` · `REVIEW` · `PO_REVIEW` ·
`DONE` · `CANCELLED`

## Bieżący sprint: Phase 1 — Project foundation

| ID | Priorytet | Zadanie | Owner | Status | Zależności |
|---|---:|---|---|---|---|
| [P1-001](tasks/P1-001-toolchain.md) | P1 | Bootstrap Go module i narzędzi | Big Pickle | `IN_PROGRESS` | Phase 0 |
| [P1-002](tasks/P1-002-configuration-logging.md) | P1 | Konfiguracja i logowanie | — | `BACKLOG` | P1-001 |
| [P1-003](tasks/P1-003-sqlite-foundation.md) | P0 | SQLite, pragmas i migracje | — | `BACKLOG` | P1-001 |
| [P1-004](tasks/P1-004-health-endpoint.md) | P1 | Endpoint health | — | `BACKLOG` | P1-002, P1-003 |
| [P1-005](tasks/P1-005-makefile.md) | P1 | Makefile jako interfejs projektu | — | `BACKLOG` | P1-001 |
| [P1-006](tasks/P1-006-docker-compose.md) | P1 | Dockerfile i Compose | — | `BACKLOG` | P1-002, P1-003, P1-004, P1-005 |
| [P1-007](tasks/P1-007-phase1-validation.md) | P1 | Walidacja i pomiary Phase 1 | — | `BACKLOG` | P1-001–P1-006 |

## Rejestr blockerów i decyzji

| Data | Zadanie | Typ | Opis | Decyzja/owner |
|---|---|---|---|---|
| — | — | — | Brak zgłoszonych blockerów | — |

## Historia zmian tablicy

| Data | Zmiana | Autor |
|---|---|---|
| 2026-11-25 | Utworzono tablicę i backlog Phase 1 | OpenCode |
| 2026-11-25 | Przydzielono P1-001 agentowi Big Pickle | OpenCode |
