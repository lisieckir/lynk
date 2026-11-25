# P1-004 — Endpoint health

| Pole | Wartość |
|---|---|
| Status | `BACKLOG` |
| Priorytet | `P1` |
| Owner | — |
| Faza | Phase 1 |
| Zależności | P1-002, P1-003 |

## Cel

Udostępnić bezpieczny endpoint gotowości używany lokalnie i przez Compose.

## Zakres

`GET /health`, sprawdzenie dostępności bazy, deterministyczne statusy 2xx/5xx,
brak autoryzacji oraz brak ujawniania sekretów.

## Kryteria akceptacji

- [ ] Zdrowa aplikacja zwraca 2xx.
- [ ] Niedostępna baza daje 5xx.
- [ ] Odpowiedź nie ujawnia ścieżek, tokenów ani szczegółów sekretów.
- [ ] Istnieją testy HTTP i test ścieżki błędu.

## Weryfikacja

Testy handlera oraz `curl`/klient HTTP przeciwko uruchomionej aplikacji.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Oczekuje na warstwę konfiguracji i DB |

## Blockery

Brak.
