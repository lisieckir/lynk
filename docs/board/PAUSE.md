# Stan projektu — pauza przed restartem

Data: 2026-11-25

## Stan główny

Ostatni wypchnięty commit przed pauzą:

```text
285e356 chore: send bookmark domain to review
```

Walidacja Phase 1, Docker foundation i start P2-001 są już na `main`; sprawdź
`git log --oneline` po wznowieniu.

## Zadania

### Phase 1

P1-001–P1-006 są zakończone. P1-007 pozostaje `BLOCKED`, ale nie przez brak
narzędzi: Docker Engine/Compose są dostępne, a `golangci-lint v2.15.0` można
uruchomić poza repozytorium (checksum SHA-256 zweryfikowany). Blokerem jest
14 zgłoszeń domyślnego lintu (12 `errcheck`, 2 `staticcheck`), przez co
`make check` bez waivera kończy się `FAIL`; `make test` i `make test-race`
przechodzą. Szczegóły i dokładny następny krok są w
`docs/board/tasks/P1-007-phase1-validation.md`.

### Phase 2

P2-001 jest `PAUSED`, nie jest zintegrowane i nie jest zakończone.

Ostatni review zaakceptował:

* naprawę deadlocka `List` przy `MaxOpenConns(1)`;
* ownership i transakcje;
* FTS5/tag synchronization;
* duplicate semantics;
* URL schemes i większość normalizacji.

Pozostała ostatnia poprawka zgodności z upstream Linkding:

1. port `:0` — upstream pomija port o wartości zero;
2. malformed query escape, np. `?a=%ZZ` — upstream toleruje i re-enkoduje
   wartość do `a=%25ZZ`.

Po tej poprawce wymagany jest ponowny review, potem selektywna integracja
worktree i pełna walidacja.

## Worktree do wznowienia

```text
/home/fox/.local/share/opencode/worktree/e89f91/p2-001-bookmark-domain
```

Worktree zawiera niedokończone, niezatwierdzone zmiany P2-001. Nie kopiować
ich do `main` bez ponownego sprawdzenia `git status` i wyniku review.

## Procedura wznowienia

1. Sprawdź `git status`, `git log --oneline` i `git worktree list`.
2. Odczytaj ten plik oraz `docs/board/STATUS.md`.
3. Sprawdź, czy agent P2-001 pozostawił zmiany w opisanym worktree.
4. Dokończ dwa edge-case’y URL.
5. Uruchom niezależny review.
6. Dopiero po `APPROVE` zintegruj P2-001, zaktualizuj board i wykonaj push.
