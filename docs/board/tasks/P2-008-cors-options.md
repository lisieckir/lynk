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

- [ ] Dozwolone origin dostaje poprawny preflight bez ujawnienia danych.
- [ ] Niedozwolone origin nie dostaje CORS headers.
- [ ] `Authorization` jest jawnie dozwolony tylko w skonfigurowanej polityce.
- [ ] `/health` zachowuje publiczny kontrakt.
- [ ] `make check`, test/race, compatibility i build przechodzą.
- [ ] Niezależny security review APPROVE.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Hy4 Flash Free | Przydzielono follow-up po P2-007 |
