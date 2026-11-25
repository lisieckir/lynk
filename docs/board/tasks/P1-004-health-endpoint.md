# P1-004 — Endpoint health

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P1` |
| Owner | Big Pickle |
| Faza | Phase 1 |
| Zależności | P1-002, P1-003 |

## Cel

Udostępnić bezpieczny endpoint gotowości używany lokalnie i przez Compose.

## Zakres

`GET /health`, sprawdzenie dostępności bazy, deterministyczne statusy 2xx/5xx,
brak autoryzacji oraz brak ujawniania sekretów.

## Kryteria akceptacji

- [x] Zdrowa aplikacja zwraca 2xx.
- [x] Niedostępna baza daje 5xx.
- [x] Odpowiedź nie ujawnia ścieżek, tokenów ani szczegółów sekretów.
- [x] Istnieją testy HTTP i test ścieżki błędu.

## Weryfikacja

Testy handlera oraz `curl`/klient HTTP przeciwko uruchomionej aplikacji.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Oczekuje na warstwę konfiguracji i DB |
| 2026-11-25 | `IN_PROGRESS` | Big Pickle | Rozpoczęto implementację endpointu health |
| 2026-11-25 | `DONE` | Flint 40B Free | Review zaakceptował lifecycle, timeout readiness i testy regresyjne |

## Blockery

Brak.
