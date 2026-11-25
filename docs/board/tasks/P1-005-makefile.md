# P1-005 — Makefile jako interfejs projektu

| Pole | Wartość |
|---|---|
| Status | `REVIEW` |
| Priorytet | `P1` |
| Owner | Tidepool Alpha Free |
| Faza | Phase 1 |
| Zależności | P1-001 |

## Cel

Utworzyć bezpieczny, czytelny Makefile zgodny z `docs/makefile-plan.md` i
sekcją 9 `SPEC.md`.

## Kryteria akceptacji

- [ ] `make help` opisuje każdy publiczny target.
- [ ] Działają targety development, test, build, database, health i Docker
      przewidziane dla aktualnego etapu.
- [ ] Użyto `.PHONY`, zmiennych i fail-fast.
- [ ] `clean`, `docker-down` i `docker-clean` nie usuwają danych persistent.
- [ ] `VERSION`, image/tag i `COMPOSE` są konfigurowalne.
- [ ] Nie ma obowiązkowego Node.js.

## Weryfikacja

Uruchomić `make help`, `make check` oraz testy destrukcyjnych granic z
tymczasową bazą i wolumenem.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Plan w `docs/makefile-plan.md` |
| 2026-11-25 | `IN_PROGRESS` | Tidepool Alpha Free | Rozpoczęto implementację Makefile |
| 2026-11-25 | `REVIEW` | OpenCode | Makefile gotowy do niezależnego review |

## Blockery

Brak.
