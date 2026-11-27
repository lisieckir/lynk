# Lynk Board — status zbiorczy

Ostatnia aktualizacja: 2026-11-26
Właściciel tablicy: PO / koordynator projektu

## Legenda

`BACKLOG` · `READY` · `IN_PROGRESS` · `BLOCKED` · `REVIEW` · `PO_REVIEW` ·
`DONE` · `CANCELLED`

## Bieżący sprint: Phase 8 — Performance and release

| ID | Priorytet | Zadanie | Owner | Status | Zależności |
|---|---:|---|---|---|---|
| [P1-001](tasks/P1-001-toolchain.md) | P1 | Bootstrap Go module i narzędzi | Big Pickle | `DONE` | Phase 0 |
| [P1-002](tasks/P1-002-configuration-logging.md) | P1 | Konfiguracja i logowanie | Big Pickle | `DONE` | P1-001 |
| [P1-003](tasks/P1-003-sqlite-foundation.md) | P0 | SQLite, pragmas i migracje | Big Pickle | `DONE` | P1-001 |
| [P1-004](tasks/P1-004-health-endpoint.md) | P1 | Endpoint health | Big Pickle | `DONE` | P1-002, P1-003 |
| [P1-005](tasks/P1-005-makefile.md) | P1 | Makefile jako interfejs projektu | Tidepool Alpha Free | `DONE` | P1-001 |
| [P1-006](tasks/P1-006-docker-compose.md) | P1 | Dockerfile i Compose | Tidepool Alpha Free | `DONE` | P1-002, P1-003, P1-004, P1-005 |
| [P1-007](tasks/P1-007-phase1-validation.md) | P1 | Walidacja i pomiary Phase 1 | GPT-6 Luna | `DONE` | P1-001–P1-006 |
| [P2-001](tasks/P2-001-bookmark-domain.md) | P1 | Domena zakładek, repository i service | GPT-6 Luna | `DONE` | P1-003 |
| [P2-002](tasks/P2-002-token-auth-bootstrap.md) | P0 | Uwierzytelnianie tokenem i bootstrap CLI | Iceman Free | `DONE` | P1-003, P2-001 |
| [P2-003](tasks/P2-003-linkding-api-foundation.md) | P0 | Fundament API Linkdinga | Hy4 Flash Free | `DONE` | P1-003, P2-001, P2-002 |
| [P2-004](tasks/P2-004-linkding-search-mutations.md) | P0 | Search, pagination i mutacje bookmarków Linkdinga | MiMo-V2.6-Flash Free | `DONE` | P2-003 |
| [P2-005](tasks/P2-005-extension-contract-replay.md) | P0 | Extension contract replay i parity pól | Nemotron Lightning Free | `DONE` | P2-004 |
| [P2-006](tasks/P2-006-metadata-ssrf.md) | P0 | Asynchroniczne metadata extraction i SSRF protection | Iceman Free | `DONE` | P2-005 |
| [P2-007](tasks/P2-007-release-e2e.md) | P0 | Release E2E i deployment validation | Big Pickle | `DONE` | P2-006 |
| [P2-008](tasks/P2-008-cors-options.md) | P1 | CORS i OPTIONS dla API | Hy4 Flash Free | `DONE` | P2-007 |
| [P2-009](tasks/P2-009-web-interface.md) | P0 | Web interface MVP | Iceman Free | `DONE` | P2-008 |
| [P2-010](tasks/P2-010-netscape-import-export.md) | P0 | Netscape HTML import/export | Hy4 Flash Free | `DONE` | P2-009 |
| [P2-011](tasks/P2-011-performance-release.md) | P0 | Performance, resource profile i final release gate | Big Pickle | `DONE` | P2-010 |
| [P2-012](tasks/P2-012-accounts-admin-tokens.md) | P0 | Konta, role admina i tokeny API | Iceman Free | `BLOCKED` | P2-011 |

## Rejestr blockerów i decyzji

| Data | Zadanie | Typ | Opis | Decyzja/owner |
|---|---|---|---|---|
| 2026-11-25 | P1-001 | Warunek akceptacji | Review zaakceptował implementację warunkowo; pin Go/SQLite wymaga potwierdzenia w P1-003 | Koordynator + PO |
| 2026-11-25 | P1-007 | Wynik lintu | `golangci-lint v2.15.0` zweryfikowany; 14 issue'ów naprawionych, `make check` bez waivera PASS | PO / developer P1-007 |
| 2026-11-25 | P2-002 | Decyzja PO | Auth + bootstrap CLI jako osobny slice; pełne endpointy API później | PO |
| 2026-11-25 | P2-002 | Akceptacja | Auth, revoke, middleware, testy i niezależny review PASS | PO |
| 2026-11-25 | P2-002 | Blocker | 6 nowych `errcheck` findings po integracji; funkcjonalny review PASS, lint wymaga poprawki | PO |
| 2026-11-25 | P2-002 | Akceptacja | Finalny review APPROVE; `make check`, build, test i race PASS | PO |
| 2026-11-25 | P2-003 | Decyzja PO | Start API od DTO/routera i minimalnego flow klienta; search/pagination rozbite osobno | PO |
| 2026-11-25 | P2-003 | Przydzielenie | Hy4 Flash Free pracuje w izolowanym worktree | PO |
| 2026-11-25 | P2-004 | Decyzja PO | Search/pagination i mutacje jako kolejny krytyczny slice extension compatibility | PO |
| 2026-11-25 | P2-004 | Review pending | Commit `e617a1a` gotowy; integracja zaparkowana do następnego dnia | PO |
| 2026-11-26 | P2-004 | Akceptacja | Re-review APPROVE; search, pagination, mutations, overflow i docs PASS | PO |
| 2026-11-25 | P2-005 | Decyzja PO | Replay/parity przed metadata i realnym E2E; SingleFile odroczone | PO |
| 2026-11-26 | P2-005 | Przydzielenie | Nemotron Lightning Free pracuje po zamknięciu P2-004 | PO |
| 2026-11-26 | P2-005 | Akceptacja | Finalny review APPROVE; replay, parity, tags pagination i pełna walidacja PASS | PO |
| 2026-11-26 | P2-006 | Decyzja PO | Metadata wymagane przed release; security-critical slice z obowiązkowym security review | PO |
| 2026-11-26 | P2-006 | Request changes | Security review wykrył high-severity parser/SSRF/shutdown issues oraz 4 lint findings | Big Pickle |
| 2026-11-26 | P2-006 | Request changes | Re-review potwierdził parser panic, 3 lint findings, persistence/lifecycle/logging gaps | Big Pickle |
| 2026-11-26 | P2-006 | Akceptacja | Security re-review APPROVE, `make check` 0 issues, test/race/build/integration PASS, PO_REVIEW PASS | PO |
| 2026-11-26 | P2-007 | Decyzja PO | Firefox/release gate po domknięciu metadata i API | PO |
| 2026-11-26 | P2-007 | Wynik E2E | Realny Firefox 161.0 + oficjalny XPI 1.15.1: profile/tags/check/save/merge/search/delete PASS przez logging proxy | PO |
| 2026-11-26 | P2-007 | Wynik walidacji | Pełna macierz Go/lint/test/race/compat/integration/release oraz Docker build/health/restart/recreate/persistence PASS | PO |
| 2026-11-26 | P2-007 | Release gap | Brak CORS nagłówków; `OPTIONS` bez tokenu `401`, z tokenem `405 Allow: GET, POST`. Oficjalne rozszerzenie działa przez host permissions | PO — decyzja o sign-off lub osobnym tasku |
| 2026-11-26 | P2-007 | Zastrzeżenie | Brak dedykowanego `goleak` assertion; create nieblokujący, testy worker i race PASS, restart czyści shutdown | PO |
| 2026-11-26 | P2-007 | Akceptacja PO | Firefox/XPI, Docker, proxy auth i persistence PASS; CORS gap przeniesiony do P2-008 | PO |
| 2026-11-25 | P2-003 | Akceptacja | Finalny review APPROVE; API contract, compatibility tests, lint i race PASS | PO |
| 2026-11-26 | P2-007 zaakceptowane; utworzono P2-008 hardening task | PO |
| 2026-11-26 | P2-008 | Security review | `6f0fc95` APPROVE, severity none; CORS/config/auth matrix PASS | Big Pickle |
| 2026-11-26 | P2-008 | Akceptacja | Zintegrowane `7ac47c5`; pełna walidacja i PO_REVIEW PASS | PO |
| 2026-11-26 | P2-009 | Decyzja PO | Web interface jest brakującym wymaganiem Definition of Done; security/session scope obowiązkowy | PO |
| 2026-11-26 | P2-009 | Request changes | Security review wykrył lint blocker, brak rate limitu/bounded sessions, logout CSRF gap, braki testów i duplicate-create overwrite | Big Pickle |
| 2026-11-26 | P2-009 | Request changes | Follow-up `a62e0b5` nadal bez exact pinned check, konfigurowalnych limitów i pełnego test matrix | PO |
| 2026-11-26 | P2-009 | Request changes | `80197f1` ma linter 0 issues, lecz env parsing i exhaustive security/flow tests nadal brakują | PO |
| 2026-11-26 | P2-009 | Review pending | `0c27d07` ma env parsing i pinned check PASS; final security review ma potwierdzić blokery oraz zakres testów | PO |
| 2026-11-26 | P2-009 | Akceptacja | Security `APPROVE`, full matrix PASS, cookie policy test/docs added; PO_REVIEW PASS | PO |
| 2026-11-26 | P2-010 | Decyzja PO | Netscape import/export jest wymaganiem Definition of Done; bounded CLI i round-trip tests obowiązkowe | PO |
| 2026-11-26 | P2-010 | Review pending | `043cba0` pełna macierz PASS; niezależny security/correctness review w toku | PO |
| 2026-11-26 | P2-010 | Request changes | Symlinked-parent DB overwrite HIGH i cumulative field-cap bypass MEDIUM; poprawki zlecone | Big Pickle |
| 2026-11-26 | P2-010 | Review pending | `9dcd3da` fixes PASS; niezależna re-review w toku | PO |
| 2026-11-26 | P2-010 | Akceptacja | Re-review `APPROVE`, full matrix PASS, integrated `4d7fee8`/`1ebbb30`; PO_REVIEW PASS | PO |
| 2026-11-26 | P2-011 | Decyzja PO | Phase 8 performance/resource/release gate przed stable release | PO |
| 2026-11-26 | P2-011 | Review pending | Benchmark, resource and Docker evidence PASS; independent release review w toku | PO |
| 2026-11-26 | P2-011 | Akceptacja PO | Release review `APPROVE`, FTS differential PASS, full matrix/Docker/resource evidence PASS | PO |
| 2026-11-27 | P2-012 | Decyzja PO | Account signup, first-admin role, admin user management and UI API-token creation required | PO |
| 2026-11-27 | P2-012 | Review pending | Implementation and basic matrix PASS; independent security review required | PO |
| 2026-11-27 | P2-012 | Request changes | Security review found three HIGH issues, session/revoke/audit gaps and missing required tests | Big Pickle |
| 2026-11-27 | P2-012 | Request changes | `cdbf56a` fixes core HIGHs; remaining admin token UI/audit/rate-limit/tests/Docker gaps assigned | PO |
| 2026-11-27 | P2-012 | Request changes | Second-pass UI/logging changes uncommitted; critical regressions and isolated Docker smoke still required | PO |
| 2026-11-27 | P2-012 | Request changes | UI/logging committed and health smoke PASS; required account regression tests/full register-token-API flow still missing | PO |
| 2026-11-27 | P2-012 | Review pending | `8c9f144` supplies 9 acceptance tests and Docker flow PASS; final security review in progress | PO |

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
| 2026-11-25 | P2-002 zaakceptowane po poprawkach i niezależnym review | PO |
| 2026-11-25 | P2-002 domknięte po naprawie 6 findings lintu i finalnym review | PO |
| 2026-11-25 | Utworzono P2-003 fundament API Linkdinga | PO |
| 2026-11-25 | Przydzielono P2-003 agentowi Hy4 Flash Free | PO |
| 2026-11-25 | P2-003 zintegrowane po niezależnym review (`66ea32e`) | PO |
| 2026-11-25 | Przydzielono P2-004 agentowi Tidepool Alpha Free | PO |
| 2026-11-25 | P2-004 implementacja ukończona przez MiMo; zaparkowano przed review (`e617a1a`) | PO |
| 2026-11-26 | P2-004 zintegrowane (`a9b38c8`, `a7d43e4`) po niezależnym APPROVE | PO |
| 2026-11-25 | Utworzono P2-005 jako następny gate kompatybilności rozszerzenia | PO |
| 2026-11-26 | Przydzielono P2-005 agentowi Nemotron Lightning Free | PO |
| 2026-11-26 | P2-005 zintegrowane (`0d02e58`) po niezależnym APPROVE | PO |
| 2026-11-26 | Przydzielono P2-006 agentowi Iceman Free | PO |
| 2026-11-26 | P2-006 zablokowane po security review; poprawki zlecone | PO |
| 2026-11-26 | P2-006 zintegrowane (`b825ef4`, `3a3a3bf`, `d2b5c6b`) i zamknięte po security review | PO |
| 2026-11-26 | Przydzielono P2-007 agentowi Big Pickle | PO |
| 2026-11-25 | P1-007 zaakceptowane po niezależnym review; lint, testy i Docker validation PASS | PO |
| 2026-11-26 | P2-007: fast-forward do `8333591` i pełna macierz Go/lint/build PASS | Big Pickle |
| 2026-11-26 | P2-007: Docker/Compose build, health, restart, recreate i named-volume persistence PASS | Big Pickle |
| 2026-11-26 | P2-007: realny headless Firefox + oficjalny XPI 1.15.1 E2E PASS; CORS/OPTIONS gap przekazany do decyzji PO | Big Pickle |
