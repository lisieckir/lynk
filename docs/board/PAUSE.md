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
* **P2-003 — `IN_PROGRESS`**: Hy4 Flash Free implementuje DTO/router i minimalny
  kontrakt API Linkdinga w izolowanym worktree.

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
