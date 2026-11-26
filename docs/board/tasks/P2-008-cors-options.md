# P2-008 — CORS i OPTIONS dla API

| Pole | Wartość |
|---|---|
| Status | `IN_PROGRESS` |
| Priorytet | `P1` |
| Owner | Hy4 Flash Free |
| Faza | Phase 4 hardening |
| Zależności | P2-007 |

## Cel

Domknąć jawny release gap wykryty podczas P2-007: bezpieczne CORS i OPTIONS
dla browser clients, bez osłabiania auth ani ujawniania danych.

## Zakres

* polityka allowed origins z konfiguracji, bez wildcard z credentials;
* preflight OPTIONS bez tokenu tylko dla dozwolonych origin/method/headers;
* `Access-Control-Allow-*`, `Vary: Origin` i max-age;
* odrzucenie niedozwolonych origin bez sekretów;
* testy handler/proxy i dokumentacja bezpiecznych defaultów.

## Kryteria akceptacji

- [x] Dozwolone origin dostaje poprawny preflight bez ujawnienia danych.
- [x] Niedozwolone origin nie dostaje CORS headers.
- [x] `Authorization` jest jawnie dozwolony tylko w skonfigurowanej polityce.
- [x] `/health` zachowuje publiczny kontrakt.
- [x] `make check`, test/race, compatibility i build przechodzą.
- [ ] Niezależny security review APPROVE — oczekuje na reviewera.

## Wynik skrócony

Nowy pakiet `internal/cors` implementuje fail-closed allow-list: puste
`LYNK_CORS_ALLOWED_ORIGINS` (default) oznacza brak nagłówków CORS dla dowolnego
originu. Preflight `OPTIONS` jest obsługiwany bez tokenu tylko dla jawnie
skonfigurowanego originu, metody i nagłówków (`204`, `Access-Control-Allow-*`,
`Vary: Origin`, `Access-Control-Max-Age`); niedozwolony preflight dostaje `403`
z pustym body i bez nagłówków CORS. Żadnych wildcardów, `Allow-Credentials`
ani wycieku tokenu. `Authorization` wymaga jawnego wpisania w
`LYNK_CORS_ALLOWED_HEADERS`. `GET /health` pozostaje publiczne i nie jest
owijane CORS; auth API bez zmian (`401` bez tokenu).

Walidacja (bazowy commit `35c2a94`): `gofmt -l .` puste, `go vet ./...`,
`go build ./...`, `go test ./...`, `go test -race ./...`, `make test-compat`,
`make test-integration`, `make LINT_TOOL=<golangci-lint v2.15.0> check`
(0 issues), `go mod tidy -diff` puste, `go mod verify`, `make build`,
`docker compose config` — PASS. Dodano testy jednostkowe w `internal/cors`
i `internal/config`, testy handlera w `internal/server` oraz pełny flow HTTP
w `tests/integration/cors_test.go`.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Hy4 Flash Free | Przydzielono follow-up po P2-007 |
| 2026-11-26 | `IMPLEMENTED` | Hy4 Flash Free | CORS/OPTIONS z konfigurowalną allow-listą, testy i dokumentacja; pełna macierz walidacji PASS; security review w toku |
