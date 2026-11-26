# P2-007 — Release E2E i deployment validation

| Pole | Wartość |
|---|---|
| Status | `PO_REVIEW` |
| Priorytet | `P0` |
| Owner | Big Pickle |
| Faza | Phase 4 release gate |
| Zależności | P2-006 |
| Baza | `main` `8333591` (worktree zaktualizowany z `74d40b5` przez `git merge --ff-only`) |

## Cel

Zweryfikować działanie release'u z oficjalnym rozszerzeniem Firefox oraz
deploymentem Docker/Compose, ze szczególnym uwzględnieniem metadata, auth,
proxy/CORS i persistence.

## Zakres

* realny Firefox smoke: profile, tags, check metadata, save/merge, search,
  delete, archive/update jeśli klient je wywołuje;
* weryfikacja `Authorization` przez reverse-proxy-like path i CORS/OPTIONS;
* Docker build/up/health/restart/persistence po workerze metadata;
* zapisanie wersji rozszerzenia, browsera, obrazu i znanych ograniczeń;
* jeśli brak XPI/drivera, deterministyczny blocker report i request replay
  pozostają dowodem częściowym — bez fałszywego sign-off.

## Poza zakresem

Chrome E2E, SingleFile/assets/bundles oraz pełna upstream API parity.

## Kryteria akceptacji

- [x] Firefox flow przechodzi na realnym browserze z oficjalnym XPI albo
  blocker środowiskowy jest udokumentowany z reprodukcją.
- [x] Docker build, health, restart i named-volume persistence PASS.
- [x] Metadata worker nie blokuje create i kończy się bez obserwowanego
  wycieku; brak dedykowanego `goleak` assertion pozostaje zastrzeżeniem.
- [ ] Proxy/CORS behavior sprawdzone: proxy PASS, natomiast CORS/OPTIONS jest
  jawnym release gapem wymagającym decyzji PO.
- [x] `make check`, test/race, compatibility, integration i build przechodzą.
- [ ] PO akceptuje release gate albo zapisuje konkretne blocker/next action.

## Wynik skrócony

| Obszar | Wynik | Dowód |
|---|---|---|
| Macierz Go/lint/build | PASS | `gofmt`, `vet`, `build`, `test`, `race`, `test-compat`, `test-integration`, `check`, `tidy -diff`, `verify`, `release`; szczegóły w [development.md](../../development.md) |
| Docker/Compose | PASS | build, up, health, API smoke, restart, down/recreate, named-volume persistence, cleanup |
| Firefox + oficjalne rozszerzenie | PASS | Firefox 161.0 headless, XPI 1.15.1, realne żądania przez logging proxy; szczegóły w [api-compatibility.md](../../api-compatibility.md) |
| Proxy `Authorization` | PASS | nagłówek przekazywany przez proxy; relative `next` bez internal host |
| CORS/OPTIONS | **GAP** | brak `Access-Control-*`; `OPTIONS` bez tokenu `401`, z tokenem `405 Allow: GET, POST` |
| PO sign-off | PENDING | status `PO_REVIEW` |

Nie zmieniono kodu produkcyjnego. Wszystkie zmiany w tym zadaniu są
dokumentacją release validation.

## Środowisko i wersje

| Element | Wersja / identyfikator |
|---|---|
| Go | `go1.27.4-X:nodwarf5 linux/amd64` |
| `golangci-lint` | `2.15.0` (build `25c5369a`, outside repo) |
| Docker Engine / Compose | `29.10.0` / `5.8.0` |
| Image | `ghcr.io/lisieckir/lynk:p2-007`, `sha256:37433b97848a696f81b69e725014483c65f371862a00ccbbc25c07dbc2eec212`, 30,470,977 bytes |
| Firefox | `Mozilla Firefox 161.0` (`/usr/bin/firefox`) |
| XPI | linkding extension `1.15.1`, ID `{61a05c39-ad45-4086-946f-32adb0a40a9d}` |
| XPI SHA-256 | `e32bfc2e21c8d4035212fac3cc2eadcd78fe7016352353df0d3400342953671d` |
| XPI source | `https://addons.mozilla.org/firefox/downloads/file/4860177/linkding_extension-1.15.1.xpi` |

Rozszerzenie jest aktywne w profilu
`/home/fox/.config/mozilla/firefox/1w4228hv.default-release` (`extensions.json`
pokazuje `active: true`, `userDisabled: false`). Do E2E użyto kopii XPI w
izolowanym profilu `/tmp/opencode/p2-007/ff-profile`, bez dotykania profilu
użytkownika.

## Macierz walidacji

Uruchomiono skrypt `/tmp/opencode/p2-007/go-matrix.sh`; podsumowanie w
`/tmp/opencode/p2-007/go-summary.txt`, logi per-krok w `/tmp/opencode/p2-007/*.log`.

| Komenda | Wynik |
|---|---|
| `gofmt -l .` | PASS, brak plików |
| `go vet ./...` | PASS |
| `go build ./...` | PASS |
| `go test ./...` | PASS, w tym `internal/worker`, `internal/metadata` i `tests/integration` |
| `go test -race ./...` | PASS, w tym `tests/integration` |
| `make test-compat` | PASS |
| `make test-integration` | PASS |
| `make LINT_TOOL=<pinned v2.15.0> check` | PASS, `0 issues` |
| `go mod tidy -diff`, `go mod verify` | PASS |
| `make LINT_TOOL=<pinned v2.15.0> release` | PASS |

## Docker/Compose evidence

Uruchomiono skrypt `/tmp/opencode/p2-007/docker-matrix.sh` z izolowanym
`COMPOSE_PROJECT_NAME=lynk_p2007`, host portem `19090`, `LYNK_ENV=production`
i `LYNK_BASE_URL=http://localhost:19090`. Token został utworzony przez
`lynk token create` i nie jest zapisany w dokumentacji.

* `docker compose config` — PASS.
* `make docker-release` — PASS; image jak w tabeli wersji.
* `docker compose up -d --build` — PASS; kontener
  `lynk_p2007-lynk-1` (`23f82a2d…`) osiągnął `healthy`.
* `GET /health` — `200 {"status":"ok"}`; `make health` — PASS;
  `docker compose ps` — `Up (healthy)`, `0.0.0.0:19090->9090/tcp`.
* API przez token: `profile 200`, `tags 200`, brak tokenu `401`,
  `POST /api/bookmarks/?disable_scraping&disable_html_snapshot 201` w
  `0.0055s`, trzy markery, `GET /api/bookmarks/?q=p2-007&limit=1 200` z
  `count=3` i `next=/api/bookmarks/?limit=1&offset=1&q=p2-007`,
  `GET /api/bookmarks/check/?url=... 200` w `0.7218s` z
  `metadata` nullable i `auto_tags=[]`.
* `docker compose restart lynk` — PASS; ten sam CID ponownie `healthy`,
  `GET /api/bookmarks/1/ 200`. Logi pokazują uporządkowane
  `http server shutting down` → `shutdown complete` → ponowny start.
* `docker compose down` zachował wolumen `lynk_p2007_lynk_data`;
  `docker compose up -d --build` utworzył nowy zdrowy kontener
  (`113f0bdb…`). Po recreate `profile 200`, `detail 200`,
  `count=3` z tymi samymi URLami, a `/data/lynk.db` nie jest pusty
  (`DB_PRESENT`). Idle RSS: `7.766MiB`.
* Końcowe `docker compose down` i `docker volume rm lynk_p2007_lynk_data` —
  PASS; środowisko testowe usunięte.

## Firefox E2E evidence

Nie było `geckodriver`, `xdotool`, `xte`, `ydotool`, `wmctrl`, `scrot`,
`Xvfb` ani Python `selenium`/`playwright`. Nie jest to brak XPI: oficjalny
XPI jest dostępny i aktywny. Zamiast drivera użyto wbudowanego w Firefox
`WebDriver BiDi` (`--remote-debugging-port 9223`) oraz devtools RDP
(`--start-debugger-server ws:6000`) w headless, izolowanym profilu.
Nie była to interakcja GUI/myszą; była to realna egzekucja kodu oficjalnego
rozszerzenia w przeglądarce, z formularza popupu i przez jego własny obiekt
API.

`firefox --headless --new-instance --profile /tmp/opencode/p2-007/ff-profile
--remote-debugging-port 9223 --start-debugger-server ws:6000
--remote-allow-hosts 127.0.0.1,localhost` wystartował poprawnie
(`WebDriver BiDi listening`, `Started devtools server on 6000`). BiDi
`session.new` zwrócił `browserName=firefox`, `browserVersion=161.0`,
`moz:headless=true`. `listAddons` z RDP znalazł
`{61a05c39-ad45-4086-946f-32adb0a40a9d}` jako debuggable web extension z
`manifestURL=.../manifest.json`.

Rozszerzenie skonfigurowano na lokalny logging proxy
`http://127.0.0.1:19091` (`baseUrl`) z tokenem testowym. Proxy przekazywał
żądania do `http://127.0.0.1:19090` i logował metodę, ścieżkę, obecność
nagłówka `Authorization` (zredagowaną), `Origin`, rozmiar ciała i status.
Sekwencja z `/tmp/opencode/p2-007/proxy-requests.log`:

| Żądanie rozszerzenia | Status |
|---|---:|
| `GET /api/tags/?limit=5000`, `Authorization: Token <redacted>` | 200 |
| `GET /api/user/profile/`, `Authorization: Token <redacted>` | 200 |
| `GET /api/bookmarks/check/?url=<encoded marker>`, `Authorization: Token <redacted>` | 200 |
| `POST /api/bookmarks/?disable_scraping`, `Origin: moz-extension://e85c…`, `Authorization: Token <redacted>`, JSON 203 B | 201 |
| `POST /api/bookmarks/?disable_scraping` ponownie (merge), JSON 210 B | 201 |
| `GET /api/bookmarks/check/?url=<encoded marker>` ponownie | 200 (`bookmark.id=1`) |
| `GET /api/bookmarks/?q=p2-007-firefox&limit=5` | 200 |
| `DELETE /api/bookmarks/1/`, `Origin: moz-extension://e85c…` | 204 |

Stan popupu po submitcie: `saveState=success`, `errorMessage=""`. `check`
zwrócił `metadata={url: <marker>, title: null, description: null,
preview_image: null}` oraz `auto_tags=[]`. Po delete zewnętrzny odczyt API
zwrócił `count=0`. Log rozszerzenia: `/tmp/opencode/p2-007/firefox-e2e.log`.
Logi serwisa: `/tmp/opencode/p2-007/firefox-service.log`.

Archive/unarchive, PUT, PATCH, asset i bundle nie były wywołane, ponieważ
oficjalne rozszerzenie 1.15.1 ich nie używa; pozostają poza E2E gate'em zgodnie
z `api-compatibility.md`.

## Proxy, CORS i gap release'owy

Logging proxy na `127.0.0.1:19091` poprawnie przekazał `Authorization: Token
<redacted>` do Lynk i zwrócił `200`; pagination `next` pozostała względna
(`/api/bookmarks/?limit=1&offset=1&q=p2-007`), więc nie ujawnia wewnętrznego
hosta. To domyka część proxy acceptance criteria.

CORS/OPTIONS nie przeszło:

* `OPTIONS /api/bookmarks/` z `Origin: https://example.com`,
  `Access-Control-Request-Method: POST` i
  `Access-Control-Request-Headers: authorization,content-type` bez tokenu →
  `401`;
* ten sam `OPTIONS` z tokenem → `405`, `Allow: GET, POST`;
* `GET /api/user/profile/` z `Origin: https://example.com` i tokenem →
  `200`, ale bez żadnego nagłówka `Access-Control-*`.

W źródle nie ma obsługi `OPTIONS` ani nagłówków `Access-Control-*`; nie ma
także obsługi zaufanych nagłówków proxy typu `X-Forwarded-*`. Oficjalne
rozszerzenie działa, bo ma host permissions i wysyła token bezpośrednio, ale
webowy klient z innego originu zostanie zablokowany przez CORS. **Nie zmieniamy
kodu produkcyjnego poza zakresem zadania.** Wymagana jest decyzja PO:
przyjąć jako znany limit release'u i otworzyć osobny task CORS/OPTIONS, albo
zlecić rozwiązanie przed sign-offem.

## Metadane worker — zastrzeżenie

`POST` z `disable_scraping` zwrócił `201` w `0.0055s`, a `check` z metadanych
zwrócił `200` w `0.7218s`; create nie był blokowany przez fetch. Testy
`internal/worker` (`StopIdempotent`, `EnqueueAfterStop`) oraz cały
`go test -race ./...` przeszły, a restart kontenera zakończył się czystym
`shutdown complete`. Nie istnieje dedykowany test `goleak`, więc „brak wycieku”
jest potwierdzony obserwacyjnie, nie jako formalna asercja.

## Decyzja PO / next action

Release gate **nie jest podpisany**. Wymagana decyzja PO dla CORS/OPTIONS gapu.
Jeśli PO przyjmie znany limit, następny krok to osobny task z właścicielem dla
CORS/OPTIONS i opcjonalnie dedykowanego `goleak` assertion; P2-008 nie został
rozpoczęty.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-26 | `IN_PROGRESS` | Big Pickle | Przydzielono release validation |
| 2026-11-26 | `IN_PROGRESS` | Big Pickle | Fast-forward do `8333591`; pełna macierz Go/lint/build PASS |
| 2026-11-26 | `IN_PROGRESS` | Big Pickle | Docker build/up/health/restart/recreate/persistence PASS |
| 2026-11-26 | `IN_PROGRESS` | Big Pickle | Realny Firefox 161 + XPI 1.15.1: profile/tags/check/save/merge/search/delete PASS przez proxy |
| 2026-11-26 | `PO_REVIEW` | Big Pickle | CORS/OPTIONS udokumentowany jako release gap; oczekuje decyzji PO |
