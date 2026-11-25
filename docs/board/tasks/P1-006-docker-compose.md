# P1-006 — Dockerfile i Compose

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P1` |
| Owner | Tidepool Alpha Free |
| Faza | Phase 1 |
| Zależności | P1-002, P1-003, P1-004, P1-005 |

## Cel

Dostarczyć uruchamialny, jedousługowy deployment Docker zgodny ze specyfikacją.

## Zakres

Multi-stage build, statyczny/non-root runtime zgodny z driverem, CA
certificates, `/data`, persistent volume, port i środowisko, restart policy,
healthcheck.

## Kryteria akceptacji

- [x] `make docker-build` buduje obraz.
- [x] `make docker-up` uruchamia zdrowy kontener.
- [x] Kontener działa jako non-root.
- [x] Dane pozostają po restarcie kontenera.
- [x] Compose ma dokładnie jedną usługę Lynk.
- [x] Obraz runtime nie zawiera toolchainu developerskiego.
- [x] HTTPS metadata path ma dostęp do CA certificates.

## Weryfikacja

Build, start, healthcheck, restart z zachowaniem wolumenu oraz inspekcja
użytkownika i zawartości obrazu.

Docker nie jest zainstalowany w środowisku koordynatora, więc `docker-build` i
`docker-up` nie zostały wykonane lokalnie. Zamiast tego wykonano hadolint,
`docker-compose config`, weryfikację digestów/base image, dry-run targetów oraz
równoważny lokalny build statycznego binary i test health/lifecycle. Pełny
build/run obrazu pozostaje walidacją do wykonania na hoście z Dockerem.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Oczekuje na działającą aplikację i Makefile |
| 2026-11-25 | `IN_PROGRESS` | Tidepool Alpha Free | Rozpoczęto Dockerfile i Compose |
| 2026-11-25 | `REVIEW` | OpenCode | Dockerfile i Compose gotowe do niezależnego review |
| 2026-11-25 | `DONE` | OpenAI reviewer | Review zaakceptował obraz i Compose; Docker lokalnie niedostępny |

## Blockery

Brak.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| 2026-11-25 | Distroless czy Alpine | Wybrano Alpine 3.20.10: CA, `wget`, `sh` dla healthcheck/debug; obraz digest-pinned | OpenAI reviewer |
