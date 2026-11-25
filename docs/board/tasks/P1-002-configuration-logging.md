# P1-002 — Konfiguracja i logowanie

| Pole | Wartość |
|---|---|
| Status | `REVIEW` |
| Priorytet | `P1` |
| Owner | Big Pickle |
| Faza | Phase 1 |
| Zależności | P1-001 |

## Cel

Dostarczyć typowaną konfigurację środowiska oraz bezpieczne logowanie
strukturalne.

## Zakres

Zmienne: `LYNK_ENV`, `LYNK_HTTP_ADDR`, `LYNK_DATA_DIR`,
`LYNK_DATABASE_PATH`, `LYNK_LOG_LEVEL`, `LYNK_BASE_URL`. Należy obsłużyć
walidację, wartości domyślne, `context.Context`, graceful shutdown i
redakcję sekretów.

## Kryteria akceptacji

- [ ] Błędna konfiguracja kończy start czytelnym błędem.
- [ ] Domyślna konfiguracja jest zgodna z `SPEC.md`.
- [ ] Tokeny i nagłówki `Authorization` nie trafiają do logów.
- [ ] SIGTERM kończy serwer w kontrolowany sposób.
- [ ] Są testy jednostkowe konfiguracji i redakcji.

## Weryfikacja

Testy jednostkowe oraz uruchomienie procesu z konfiguracją poprawną i
niepoprawną.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Oczekuje na P1-001 |
| 2026-11-25 | `IN_PROGRESS` | Big Pickle | Rozpoczęto konfigurację i logowanie |
| 2026-11-25 | `REVIEW` | OpenCode | Implementacja gotowa do niezależnego review |

## Blockery

Brak technicznych. Decyzja o tekstowym formacie logów w development wymaga
akceptacji PO.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| — | Format logów w trybie development | Otwarte | — |
