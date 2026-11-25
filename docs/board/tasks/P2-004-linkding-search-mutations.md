# P2-004 — Search, pagination i mutacje bookmarków Linkdinga

| Pole | Wartość |
|---|---|
| Status | `REVIEW` |
| Priorytet | `P0` |
| Owner | Tidepool Alpha Free |
| Faza | Phase 3 API |
| Zależności | P2-003 |

## Cel

Domknąć krytyczny kontrakt bookmarków używany przez oficjalne rozszerzenie i
API Linkdinga: prawdziwy search FTS, pagination oraz pełne mutacje/archive.

## Zakres

* `q`, `limit`, `offset` oraz envelope `{count,next,previous,results}`;
* `/api/bookmarks/archived/`;
* `PUT` i `PATCH /api/bookmarks/{id}/`;
* `POST .../archive/` i `.../unarchive/`;
* bezpieczne zapytania FTS5 obejmujące URL, tytuł, opis/notatki i tagi;
* owner scoping, collision/validation mapping i puste `204`;
* realne compatibility tests zgodne z `docs/api-compatibility.md`.

## Poza zakresem

Metadata fetching, HTML snapshots, assets/bundles, sharing, rate limiting i
browser-extension E2E.

## Kryteria akceptacji

- [ ] `q` działa po FTS5 i nie pozwala na raw SQL/FTS injection.
- [ ] `limit`/`offset` są walidowane i odwzorowane w `count,next,previous,results`.
- [ ] Archived list i archive/unarchive działają z owner scoping.
- [ ] PUT wymaga URL; PATCH jest częściowy; collision zwraca `400`.
- [ ] Mutacje zwracają kontraktowe `200`/`204`, a nieudane ownership `404`.
- [ ] Compatibility tests pokrywają search, pagination, update i archive.
- [ ] `make check`, `make test-compat`, `go test -race ./...`, build i verify
  przechodzą.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-25 | `IN_PROGRESS` | Tidepool Alpha Free | Przydzielono implementację w izolowanym worktree |
| 2026-11-25 | `REVIEW` | MiMo-V2.6-Flash Free | Commit `e617a1a`; search/pagination/mutations PASS, oczekuje niezależnego review |

## Ryzyka i decyzje

Linkding używa request-derived `next`/`previous`; implementacja może zwracać
`null` na końcach stron, ale musi zachować klucze i prawidłowy limit.

## Zaparkowany rezultat

Worktree: `/home/fox/.local/share/opencode/worktree/e89f91/p2-004-search-mutations`

Commit `e617a1a6aaa9811d2d0fd0c04daff787e4d20412` jest zweryfikowany przez
agenta: testy, race, vet, build, `make test-compat`, `make check` z pinned
lintem oraz `go mod verify` przechodzą. Integracja i niezależny review są
zaplanowane na następne wznowienie.
