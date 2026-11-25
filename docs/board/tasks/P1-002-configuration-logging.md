# P1-002 — Konfiguracja i logowanie

| Pole | Wartość |
|---|---|
| Status | `DONE` |
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

- [x] Błędna konfiguracja kończy start czytelnym błędem.
- [x] Domyślna konfiguracja jest zgodna z `SPEC.md`.
- [x] Tokeny i nagłówki `Authorization` nie trafiają do logów.
- [x] SIGTERM kończy serwer w kontrolowany sposób.
- [x] Są testy jednostkowe konfiguracji i redakcji.

## Weryfikacja

Testy jednostkowe oraz uruchomienie procesu z konfiguracją poprawną i
niepoprawną.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `BACKLOG` | — | Oczekuje na P1-001 |
| 2026-11-25 | `IN_PROGRESS` | Big Pickle | Rozpoczęto konfigurację i logowanie |
| 2026-11-25 | `REVIEW` | OpenCode | Implementacja gotowa do niezależnego review |
| 2026-11-25 | `DONE` | Flint 40B Free | Review zaakceptował poprawkę ścieżek w logu startowym |

## Blockery

Brak.

## Decision log

| Data | Decyzja/pytanie | Status | Akceptacja PO |
|---|---|---|---|
| 2026-11-25 | Format logów w trybie development | JSON w production, tekstowy `slog` w development; zaakceptowano jako czytelny format developerski | PO / koordynator |
