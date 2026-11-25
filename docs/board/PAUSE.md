# Stan projektu — aktualny

Data: 2026-11-25

## Stan główny

`main` zawiera P2-001 w commicie `5d265bf` oraz wyniki Docker validation.
P2-001 jest zakończone i zaakceptowane po niezależnym review.

## Zadania

* **P1-007 — `BLOCKED`**: Docker/Compose i `golangci-lint v2.15.0` są
  dostępne, ale `make check` wykrywa 14 realnych problemów (12 `errcheck`,
  2 `staticcheck`). Testy i race przechodzą.
* **P2-002 — `IN_PROGRESS`**: Iceman Free implementuje token auth i bootstrap
  CLI w worktree:
  `/home/fox/.local/share/opencode/worktree/e89f91/p2-002-auth`

## Decyzje P2-002

* `lynk token create` drukuje surowy token tylko raz.
* Token ma 40 znaków hex; w bazie przechowywany jest wyłącznie SHA-256 digest
  i prefix.
* Akceptowane są case-insensitive schematy `Token` i `Bearer`.
* Pełne endpointy API Linkdinga będą osobnym zadaniem po review P2-002.

## Zasady wznowienia

1. Nie integrować P2-002 bez niezależnego review i pełnej macierzy testów.
2. Poprawki lintu prowadzić osobno od auth.
3. Po akceptacji P2-002 utworzyć zadanie na DTO/router/endpointy Linkdinga.
