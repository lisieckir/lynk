# Stan projektu — aktualny

Data: 2026-11-25

## Stan główny

`main` zawiera P2-001 w commicie `5d265bf` oraz wyniki Docker validation.
P2-001 jest zakończone i zaakceptowane po niezależnym review.

## Zadania

* **P1-007 — `DONE`**: Docker/Compose, `golangci-lint v2.15.0`, testy, race,
  vet i build przechodzą; wszystkie 14 zgłoszeń lintu zostało naprawionych i
  niezależnie zaakceptowanych.
* **P2-002 — `DONE`**: token auth, revoke, middleware/context i bootstrap CLI
  są zintegrowane; finalny review, linter, testy i race przeszły.
* **P2-003 — `DONE`**: DTO/router, minimalny kontrakt API Linkdinga,
  compatibility tests i owner scoping zostały zintegrowane po review.
* **P2-004 — `DONE`**: search, pagination, archive/update mutations, overflow
  guard i dokumentacja zostały zintegrowane po niezależnym APPROVE.
* **P2-005 — `DONE`**: replay kontraktu rozszerzenia, writable parity i tags
  pagination zostały zintegrowane po niezależnym APPROVE.

## Punkt wznowienia

P2-004 jest zamknięte. Następnym krokiem może być przydzielenie P2-005
extension contract replay po aktualizacji boardu.

P2-005 jest zamknięte. Następny krok to przygotowanie P2-006: SSRF-safe
metadata extraction, z obowiązkowym security review i osobnym PO_REVIEW.

* **P2-006 — `BLOCKED`**: security review wykrył high-severity parser/SSRF/
  shutdown problemy oraz 4 linter findings; poprawki są zlecone w worktree.

## Decyzje P2-002

* `lynk token create` drukuje surowy token tylko raz.
* Token ma 40 znaków hex; w bazie przechowywany jest wyłącznie SHA-256 digest
  i prefix.
* Akceptowane są case-insensitive schematy `Token` i `Bearer`.
* Pełne endpointy API Linkdinga będą osobnym zadaniem po review P2-002.

## Zasady wznowienia

1. Naprawić 6 findings lintu i ponowić `make check`.
2. Wymagać niezależnego review i pełnej macierzy testów przed integracją.
3. Nie mieszać endpointów Linkdinga z zakończonym slice'em auth.
