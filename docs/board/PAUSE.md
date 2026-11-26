# Stan projektu — aktualny

Data: 2026-11-26

## Stan główny

`main` zawiera P2-006 (`d2b5c6b` i wcześniejsze commity security review)
oraz start P2-007 (`8333591`). Release validation została wykonana w
izolowanym worktree na bazie `8333591`; nie zmieniono kodu produkcyjnego.

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
* **P2-006 — `DONE`**: SSRF-safe metadata fetcher, worker lifecycle,
  persistence/check mapping i security fixes zostały zintegrowane po APPROVE.
* **P2-007 — `PO_REVIEW`**: pełna macierz Go/lint/build, Docker
  build/health/restart/recreate/persistence oraz realny Firefox 161.0
  z oficjalnym XPI 1.15.1 PASS. Proxy `Authorization` PASS. Release gate
  czeka na decyzję PO o CORS/OPTIONS gapie.

## Punkt wznowienia

P2-007 nie jest podpisany. Wymagana jest decyzja PO: przyjąć CORS/OPTIONS jako
znany limit i otworzyć osobny task z ownerem, albo zlecić poprawkę przed
sign-offem. Nie rozpoczęto P2-008.

Dowody i szczegóły: [P2-007](tasks/P2-007-release-e2e.md),
[STATUS](STATUS.md), [api-compatibility.md](../api-compatibility.md) i
[development.md](../development.md).

## Decyzje P2-002

* `lynk token create` drukuje surowy token tylko raz.
* Token ma 40 znaków hex; w bazie przechowywany jest wyłącznie SHA-256 digest
  i prefix.
* Akceptowane są case-insensitive schematy `Token` i `Bearer`.
* Pełne endpointy API Linkdinga będą osobnym zadaniem po review P2-002.

## Zasady wznowienia

1. Nie zmieniać kodu produkcyjnego bez udowodnionego błędu i zakresu zadania.
2. Wymagać niezależnego review i pełnej macierzy testów przed integracją.
3. Nie mieszać endpointów Linkdinga z zakończonym slice'em auth.
4. CORS/OPTIONS i dedykowany `goleak` assertion wymagają osobnych decyzji PO.
