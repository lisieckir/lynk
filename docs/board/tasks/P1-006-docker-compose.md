# P1-006 — Dockerfile i Compose

| Pole | Wartość |
|---|---|
| Status | `REVIEW` |
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

- [ ] `make docker-build` buduje obraz.
- [ ] `make docker-up` uruchamia zdrowy kontener.
- [ ] Kontener działa jako non-root.
- [ ] Dane pozostają po restarcie kontenera.
- [ ] Compose ma dokładnie jedną usługę Lynk.
- [ ] Obraz runtime nie zawiera toolchainu developerskiego.
- [ ] HTTPS metadata path ma dostęp do CA certificates.

## Weryfikacja

Build, start, healthcheck, restart z zachowaniem wolumenu oraz inspekcja
użytkownika i zawartości obrazu.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Oczekuje na działającą aplikację i Makefile |
| 2026-11-25 | `IN_PROGRESS` | Tidepool Alpha Free | Rozpoczęto Dockerfile i Compose |
| 2026-11-25 | `REVIEW` | OpenCode | Dockerfile i Compose gotowe do niezależnego review |

## Blockery

Brak.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| — | Distroless czy Alpine | Otwarte | — |
