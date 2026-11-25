# Lynk Board — status zbiorczy

Ostatnia aktualizacja: 2026-11-25  
Właściciel tablicy: PO / koordynator projektu

## Legenda

`BACKLOG` · `READY` · `IN_PROGRESS` · `BLOCKED` · `REVIEW` · `PO_REVIEW` ·
`DONE` · `CANCELLED`

## Bieżący sprint: Phase 1 — Project foundation

| ID | Priorytet | Zadanie | Owner | Status | Zależności |
|---|---:|---|---|---|---|
| [P1-001](tasks/P1-001-toolchain.md) | P1 | Bootstrap Go module i narzędzi | Big Pickle | `DONE` | Phase 0 |
| [P1-002](tasks/P1-002-configuration-logging.md) | P1 | Konfiguracja i logowanie | Big Pickle | `DONE` | P1-001 |
| [P1-003](tasks/P1-003-sqlite-foundation.md) | P0 | SQLite, pragmas i migracje | Big Pickle | `DONE` | P1-001 |
| [P1-004](tasks/P1-004-health-endpoint.md) | P1 | Endpoint health | Big Pickle | `DONE` | P1-002, P1-003 |
| [P1-005](tasks/P1-005-makefile.md) | P1 | Makefile jako interfejs projektu | Tidepool Alpha Free | `DONE` | P1-001 |
| [P1-006](tasks/P1-006-docker-compose.md) | P1 | Dockerfile i Compose | Tidepool Alpha Free | `DONE` | P1-002, P1-003, P1-004, P1-005 |
| [P1-007](tasks/P1-007-phase1-validation.md) | P1 | Walidacja i pomiary Phase 1 | GPT-6 Luna | `BLOCKED` | P1-001–P1-006 |
| [P2-001](tasks/P2-001-bookmark-domain.md) | P1 | Domena zakładek, repository i service | GPT-6 Luna | `DONE` | P1-003 |
| [P2-002](tasks/P2-002-token-auth-bootstrap.md) | P0 | Uwierzytelnianie tokenem i bootstrap CLI | Iceman Free | `IN_PROGRESS` | P1-003, P2-001 |

## Rejestr blockerów i decyzji

| Data | Zadanie | Typ | Opis | Decyzja/owner |
|---|---|---|---|---|
| 2026-11-25 | P1-001 | Warunek akceptacji | Review zaakceptował implementację warunkowo; pin Go/SQLite wymaga potwierdzenia w P1-003 | Koordynator + PO |
| 2026-11-25 | P1-007 | Wynik lintu | `golangci-lint v2.15.0` dostępny poza repozytorium; `make check` bez waivera FAIL na 14 issue'ach (12 `errcheck`, 2 `staticcheck`); `test`/`test-race` PASS | PO / developer P1-007 |
| 2026-11-25 | P2-002 | Decyzja PO | Auth + bootstrap CLI jako osobny slice; pełne endpointy API później | PO |

## Historia zmian tablicy

| Data | Zmiana | Autor |
|---|---|---|
| 2026-11-25 | Utworzono tablicę i backlog Phase 1 | OpenCode |
| 2026-11-25 | Przydzielono P1-001 agentowi Big Pickle | OpenCode |
| 2026-11-25 | P1-001 przekazano do niezależnego code review | OpenCode |
| 2026-11-25 | P1-001 review zaakceptowane warunkowo; oczekuje na walidację P1-003 | Flint 40B Free |
| 2026-11-25 | Przydzielono P1-003 agentowi Big Pickle | OpenCode |
| 2026-11-25 | P1-003 zaakceptowane; pin SQLite zweryfikowany, P1-001 zamknięte | Flint 40B Free |
| 2026-11-25 | Przydzielono P1-002 agentowi Big Pickle oraz P1-005 agentowi Tidepool Alpha Free | OpenCode |
| 2026-11-25 | P1-002 przekazano do niezależnego review | OpenCode |
| 2026-11-25 | P1-005 przekazano do niezależnego review | OpenCode |
| 2026-11-25 | P1-002 zaakceptowane po poprawce logowania ścieżek | Flint 40B Free |
| 2026-11-25 | P1-005 zaakceptowane po poprawkach bezpieczeństwa restore | Flint 40B Free |
| 2026-11-25 | Przydzielono P1-004 agentowi Big Pickle | OpenCode |
| 2026-11-25 | P1-004 zaakceptowane po poprawkach lifecycle i timeoutu readiness | Flint 40B Free |
| 2026-11-25 | Przydzielono P1-006 agentowi Tidepool Alpha Free | OpenCode |
| 2026-11-25 | P1-006 przekazano do niezależnego review | OpenCode |
| 2026-11-25 | P1-006 zaakceptowane; Alpine wybrane jako runtime | OpenAI reviewer |
| 2026-11-25 | Przydzielono P1-007 agentowi GPT-6 Luna | OpenCode |
| 2026-11-25 | P1-007 zablokowane przez brak Docker Engine/Compose i golangci-lint | GPT-6 Luna |
| 2026-11-25 | P2-001 uruchomione równolegle; P1-007 pozostaje blockerem release validation | OpenCode |
| 2026-11-25 | P2-001 przekazano do niezależnego review | OpenCode |
| 2026-11-25 | P2-001 wstrzymane przed ostatnią poprawką URL; komputer operatora będzie restartowany | OpenCode |
| 2026-11-25 | P2-001 zaakceptowane po poprawkach URL i review końcowym | GPT-6 Luna |
| 2026-11-25 | Docker/Compose dostępne; wznowiono P1-007 walidację kontenera | OpenCode |
| 2026-11-25 | P1-007: build, health, restart, recreate i named-volume persistence PASS; lint nadal blokuje | OpenCode |
| 2026-11-25 | P1-007: `golangci-lint v2.15.0` zweryfikowany i uruchomiony poza repo; `make check` FAIL na 14 issue'ach lintu, testy PASS | Big Pickle |
| 2026-11-25 | Utworzono i przydzielono P2-002 agentowi Iceman Free | OpenCode |
