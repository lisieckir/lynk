# P2-006 — Asynchroniczne metadata extraction i SSRF protection

| Pole | Wartość |
|---|---|
| Status | `BLOCKED` |
| Priorytet | `P0` |
| Owner | Iceman Free |
| Faza | Phase 6 capability pulled forward for MVP compatibility |
| Zależności | P2-005 |

## Cel

Dodać bezpieczny, ograniczony fetcher metadata dla bookmarków oraz wzbogacić
`/api/bookmarks/check/` bez uzależniania zapisu bookmarka od sieci.

## Zakres

* `internal/metadata` z HTML title, meta description, OpenGraph title/description
  i favicon URL;
* HTTP(S)-only, timeouty, limit odpowiedzi, limit redirectów;
* DNS/address validation przed połączeniem i po każdym redirect;
* blokada localhost, loopback, private, link-local i cloud metadata ranges;
* ochrona przed DNS rebinding przez custom `DialContext`;
* bounded asynchronous worker/queue; create nie czeka na metadata;
* migracja/przechowanie metadata w bookmarku i DTO/check mapping;
* `disable_scraping` wyłącza fetch;
* testy SSRF, redirectów, oversized responses, timeouts, malformed HTML i
  worker shutdown, bez dostępu do publicznej sieci.

## Poza zakresem

SingleFile, snapshots, assets/bundles, public sharing, browser E2E i pełna
metadata parity z zewnętrznymi serwisami.

## Kryteria akceptacji

- [ ] Zablokowane są wszystkie adresy wymagane przez SPEC, także po redirect.
- [ ] Fetcher akceptuje wyłącznie HTTP/HTTPS i ma timeout/size/redirect limits.
- [ ] DNS rebinding nie pozwala ominąć walidacji adresu.
- [ ] HTML parser zwraca oczekiwane title/description/OG/favicon dane.
- [ ] Create nie czeka na fetch ani nie failuje przez błąd sieci.
- [ ] `disable_scraping` nie enqueue’uje zadania.
- [ ] `/check/` zachowuje stabilny JSON i nie wykonuje niekontrolowanego requestu.
- [ ] Worker ma bounded concurrency, cancellation i graceful shutdown.
- [ ] `make check`, test/race, compatibility, build i verify przechodzą.
- [ ] Niezależny security review daje `APPROVE`; następnie obowiązkowe
  `PO_REVIEW` przed `DONE`.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Iceman Free | Przydzielono security-critical implementację |
| 2026-11-26 | `BLOCKED` | Big Pickle | Security review REQUEST CHANGES: high-severity parser, SSRF range i worker shutdown findings; linter 4 issues |

## Decyzje PO

* Prawdziwe metadata fetching jest wymagane przed release’em MVP.
* Testy używają wyłącznie lokalnych `httptest.Server` i kontrolowanych resolverów.
* Brak nowych produkcyjnych zależności bez osobnej decyzji PO.

## Blockery

Nie integrować `d516fac` przed naprawą high-severity security findings,
pełnym test matrix i ponownym niezależnym security review.
