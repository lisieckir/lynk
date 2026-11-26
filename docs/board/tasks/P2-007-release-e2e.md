# P2-007 — Release E2E i deployment validation

| Pole | Wartość |
|---|---|
| Status | `IN_PROGRESS` |
| Priorytet | `P0` |
| Owner | Big Pickle |
| Faza | Phase 4 release gate |
| Zależności | P2-006 |

## Cel

Zweryfikować działanie release'u z oficjalnym rozszerzeniem Firefox oraz
deploymentem Docker/Compose, ze szczególnym uwzględnieniem metadata, auth,
proxy/CORS i persistence.

## Zakres

* manualny Firefox smoke: profile, tags, check metadata, save/merge, search,
  delete, archive/update jeśli klient je wywołuje;
* weryfikacja `Authorization` przez reverse-proxy-like path i CORS/OPTIONS;
* Docker build/up/health/restart/persistence po workerze metadata;
* zapisanie wersji rozszerzenia, browsera, obrazu i znanych ograniczeń;
* jeśli brak XPI/drivera, deterministyczny blocker report i request replay
  pozostają dowodem częściowym — bez fałszywego sign-off.

## Poza zakresem

Chrome E2E, SingleFile/assets/bundles oraz pełna upstream API parity.

## Kryteria akceptacji

- [ ] Manualny Firefox flow przechodzi albo blocker środowiskowy jest
  udokumentowany z reprodukcją.
- [ ] Docker build, health, restart i named-volume persistence PASS.
- [ ] Metadata worker nie blokuje create i kończy się bez wycieku.
- [ ] Proxy/CORS behavior jest sprawdzone albo jawnie odroczone z ownerem.
- [ ] `make check`, test/race, compatibility, integration i build przechodzą.
- [ ] PO akceptuje release gate albo zapisuje konkretne blocker/next action.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Big Pickle | Przydzielono release validation |
