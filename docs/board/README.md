# Lynk Board — lekka Jira dla agentów

Ten katalog jest repozytoryjną tablicą zadań dla zespołu agentów
developerskich oraz Product Ownera (PO).

## Struktura

```text
docs/board/
├── README.md          # zasady pracy i workflow
├── STATUS.md          # zbiorczy status wszystkich zadań
├── tasks/
│   ├── P1-001-...
│   ├── P1-002-...
│   └── ...
└── templates/
    └── task.md        # szablon nowego zadania
```

## Role

### Product Owner

PO:

* definiuje cel i priorytet;
* zatwierdza zmiany zakresu oraz decyzje architektoniczne;
* doprecyzowuje kryteria akceptacji;
* wykonuje lub zleca końcową akceptację zadania;
* rozstrzyga konflikty między zadaniami.

### Agent koordynujący

Koordynator utrzymuje `STATUS.md`, rozdziela zadania, pilnuje zależności i
zgłasza PO blokery lub decyzje wymagające akceptacji.

### Agent developerski

Agent:

* bierze jedno zadanie na raz, chyba że PO jawnie zatwierdzi inaczej;
* czyta `SPEC.md` i dokumentację powiązaną z zadaniem;
* nie zmienia założeń architektonicznych bez wpisania decyzji do zadania i
  uzyskania akceptacji PO;
* aktualizuje status w `STATUS.md` oraz sekcję postępu w zadaniu;
* dodaje testy i dokumentację adekwatnie do zmiany;
* zgłasza blokery zamiast obchodzić wymagania po cichu.

## Statusy

Dozwolone statusy:

| Status | Znaczenie |
|---|---|
| `BACKLOG` | zadanie opisane, ale nie rozpoczęte |
| `READY` | spełnia warunki rozpoczęcia i może zostać podjęte |
| `IN_PROGRESS` | agent aktywnie nad nim pracuje |
| `BLOCKED` | praca zatrzymana przez zależność, decyzję lub problem |
| `REVIEW` | implementacja/dokumentacja gotowa do przeglądu |
| `PO_REVIEW` | wymagana akceptacja Product Ownera |
| `DONE` | kryteria akceptacji spełnione |
| `CANCELLED` | zadanie anulowane decyzją PO |
| `PAUSED` | praca świadomie wstrzymana, bez utraty kontekstu |

Przejście `IN_PROGRESS -> DONE` wymaga spełnienia wszystkich kryteriów
akceptacji. Przejście do `PO_REVIEW` jest obowiązkowe, gdy zadanie:

* zmienia zakres SPEC;
* zmienia API lub kontrakt kompatybilności;
* wymaga nowej zależności;
* zmienia decyzję architektoniczną;
* wpływa na bezpieczeństwo lub dane użytkownika.

## Zasady podejmowania zadań

1. Agent wybiera zadanie oznaczone `READY`, którego zależności są `DONE`.
2. Przed rozpoczęciem wpisuje siebie w `Owner` i zmienia status w
   `STATUS.md` na `IN_PROGRESS`.
3. Nie modyfikuje jednocześnie obszaru należącego do innego aktywnego zadania,
   chyba że zadania uzgodniono jako współdzielone.
4. Po pracy aktualizuje status, wynik, testy i ewentualne blokery.
5. Agent nie oznacza zadania jako `DONE`, jeżeli nie wykonał kryteriów
   akceptacji.
6. PO może odrzucić zadanie do `IN_PROGRESS` z komentarzem opisującym brak.
7. Przed `DONE` agent sprawdza, czy wpis w `STATUS.md` oraz historia postępu
   w pliku zadania są zgodne z rzeczywistym stanem.

## Zasady współpracy i konfliktów

* `STATUS.md` jest jedynym źródłem bieżącego statusu.
* Plik zadania jest źródłem zakresu, kryteriów i decyzji dotyczących tego
  zadania.
* Decyzje wymagające zgody PO zapisujemy w sekcji `Decision log` zadania.
* Nie kasujemy historii statusów ani komentarzy.
* W razie konfliktu zakresu zatrzymujemy pracę i ustawiamy `BLOCKED`.
* Zadania powinny być małe: zwykle jeden agent i jeden logiczny rezultat.

## Workflow dla agenta

```text
READY
  -> IN_PROGRESS
  -> REVIEW
  -> PO_REVIEW (jeśli wymagane)
  -> DONE
```

Jeżeli wystąpi problem:

```text
IN_PROGRESS -> BLOCKED -> IN_PROGRESS
```

Raport zakończenia powinien zawierać:

* co zostało wykonane;
* zmienione pliki;
* uruchomione testy/weryfikację;
* znane ograniczenia;
* decyzje oczekujące na PO.

## Dodawanie zadania

Skopiuj `templates/task.md`, nadaj identyfikator następny w kolejności i
dodaj wpis do `STATUS.md`. Zadanie powinno mieć jednoznaczny rezultat,
kryteria akceptacji i jawne zależności.

## Priorytety

* `P0` — release blocker lub bezpieczeństwo;
* `P1` — wymagane dla bieżącej fazy;
* `P2` — ważne, ale może poczekać;
* `P3` — opcjonalne lub przyszłe.

Priorytet nie zmienia kolejności faz określonej w `SPEC.md`.
