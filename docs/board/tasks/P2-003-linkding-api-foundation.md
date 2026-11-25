# P2-003 — Fundament API Linkdinga

| Pole | Wartość |
|---|---|
| Status | `READY` |
| Priorytet | `P0` |
| Owner | — |
| Faza | Phase 3 przeciągnięta do przodu decyzją PO |
| Zależności | P1-003, P2-001, P2-002 |

## Cel

Uruchomić pierwszy, kompatybilny kontrakt HTTP Linkdinga na bazie gotowej
domeny bookmarków i auth. Zadanie obejmuje DTO, router oraz minimalny zestaw
endpointów potrzebny do połączenia klienta: profile, tags, bookmark check i
podstawowe list/create/get/delete.

## Zakres

* `internal/api/linkding`: DTO, błędy JSON, router i handlery;
* canonical trailing-slash routes;
* `GET /api/user/profile/`;
* `GET /api/tags/`;
* `GET /api/bookmarks/check/`;
* `GET/POST /api/bookmarks/`;
* `GET/DELETE /api/bookmarks/{id}/`;
* auth middleware z P2-002, owner scoping i statusy `401/404/200/201/204`;
* realne testy `httptest` na SQLite i uruchomienie `make test-compat`.

## Poza zakresem

Paginacja/search FTS, update/archive, metadata fetching, HTML snapshots,
public sharing, UI i browser-extension E2E — będą osobnymi zadaniami, jeśli
nie zmieszczą się bez ryzyka w tym slice.

## Kryteria akceptacji

- [ ] DTO nie ujawniają pól wewnętrznych i mają stabilny JSON.
- [ ] Wszystkie trasy wymagają trailing slash i zachowują `/health`.
- [ ] Profile, tags, check i CRUD bookmarków działają z prawidłowym tokenem.
- [ ] Cross-user access zwraca `404`, błędne auth `401`.
- [ ] Create zwraca `201`, delete puste `204`, błędy mają JSON.
- [ ] Testy kontraktowe używają prawdziwego SQLite i `make test-compat` nie jest
  już no-opem.
- [ ] `go test -race ./...`, `go vet ./...`, `make check` i build przechodzą.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `READY` | PO | Zadanie utworzone po zamknięciu auth foundation |

## Decyzje

Pełna zgodność pagination/search oraz pozostałe endpointy zostaną rozbite na
następne zadania po ocenie rozmiaru tego slice.
