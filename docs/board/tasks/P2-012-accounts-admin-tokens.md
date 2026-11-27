# P2-012 — Konta, role admina i tokeny API

| Pole | Wartość |
|---|---|
| Status | `DONE` |
| Priorytet | `P0` |
| Owner | Iceman Free |
| Faza | Phase 3/5 auth hardening |
| Zależności | P2-011 |

## Cel

Umożliwić użytkownikowi założenie konta, zalogowanie się hasłem, utworzenie
tokenu API do oficjalnej wtyczki oraz bezpieczne zarządzanie kontami przez
pierwszego administratora.

## Decyzje funkcjonalne

* Pierwsza rejestracja w pustej bazie tworzy konto `admin`.
* Po utworzeniu pierwszego konta publiczna rejestracja jest zamknięta;
  administrator tworzy kolejne konta.
* Hasła są przechowywane przez nowoczesny password hash, nigdy plaintext.
* Token API jest pokazywany tylko raz po utworzeniu; baza przechowuje digest.
* Istniejące tokenowe API i CLI pozostają kompatybilne.
* Dezaktywacja użytkownika blokuje login, sesję i wszystkie jego tokeny, bez
  kasowania jego bookmarków.

## Zakres

* migracja roli `admin/user` i aktywności użytkownika; pierwszy istniejący
  użytkownik zostaje adminem;
* `/register`, login username/password oraz bezpieczne session/CSRF flow;
* `/settings/tokens`: create/list/revoke własnych tokenów, raw token only once;
* `/admin/users`: list/create/disable/enable/reset password/revoke tokenów;
* admin authorization na każdym endpointcie i owner scoping bookmarków;
* bounded input, rate limiting rejestracji/loginu, security headers i audit-safe
  logs bez haseł/tokenów;
* testy migracji, hashowania, first-admin race, auth/session invalidation,
  token lifecycle, IDOR/admin authorization i realnego plugin token flow.

## Kryteria akceptacji

- [x] Pusta baza pozwala utworzyć pierwsze konto i nadaje mu admin.
- [x] Kolejna publiczna rejestracja jest zamknięta; admin może utworzyć usera.
- [x] Login hasłem działa, hash nie ujawnia plaintextu, sesje mają CSRF.
- [x] Użytkownik tworzy token jednorazowo i może podpiąć nim wtyczkę.
- [x] API odrzuca tokeny użytkownika disabled/revoked.
- [x] Admin-only routes są odporne na IDOR i zwykły user dostaje 403.
- [x] Disable invaliduje sesje/tokeny bez kasowania bookmarków.
- [x] Migracja istniejącej bazy wybiera pierwszego usera jako admin.
- [x] `make check`, test/race, compat, integration, build i Docker PASS.
- [x] Niezależny security review `APPROVE` oraz PO_REVIEW.

## Postęp

| Data | Status | Agent | Opis |
|---|---|---|---|
| 2026-11-27 | `IN_PROGRESS` | Iceman Free | Przydzielono account/admin/token lifecycle |
| 2026-11-27 | `REVIEW` | Iceman Free | `70c96fe`/`3f58478` implementują lifecycle; niezależny security review w toku |
| 2026-11-27 | `BLOCKED` | Big Pickle | REQUEST CHANGES: register CSRF HIGH, concurrent first-admin HIGH, token URL leak HIGH, stale sessions/admin revoke/audit/tests gaps |
| 2026-11-27 | `BLOCKED` | PO | `cdbf56a` fixes HIGH/session epoch, but admin token UI, audit logs, registration limit, full regressions and Docker smoke remain |
| 2026-11-27 | `BLOCKED` | PO | Second pass exists only uncommitted; critical regression tests and isolated Docker smoke remain outstanding |
| 2026-11-27 | `BLOCKED` | PO | `8eb8abc`/`a4c2abf` committed UI/logging/docs and health smoke, but acceptance-critical account regression tests and full flow smoke remain |
| 2026-11-27 | `REVIEW` | Hy4 Flash Free | `8c9f144` adds acceptance tests, migration tests and real Docker register/login/token/API/restart smoke; final security review pending |
| 2026-11-27 | `DONE` | PO | Final security `APPROVE`, full matrix and Docker account flow PASS; PO_REVIEW PASS |

## Blockery

Brak blockerów. P2-012 jest zintegrowane po niezależnym security `APPROVE`.
