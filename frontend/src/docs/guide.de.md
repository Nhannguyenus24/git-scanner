# Verwendung von GitScanner

GitScanner klont die gesamte Historie eines Repositories, durchsucht jeden eindeutigen Blob und
meldet gefundene Anmeldedaten — sowohl im aktuellen Arbeitsbaum als auch an beliebiger Stelle in der Vergangenheit.
Diese Seite ist eine Kurzanleitung für das Dashboard.

## Verbindung

Die API-Adresse befindet sich in der oberen Leiste; der Punkt daneben leuchtet grün, wenn das
Dashboard die API erreichen kann. Tragen Sie Ihren eigenen API-Host ein, falls Sie nicht den
Standard `http://localhost:8080` verwenden. Adresse, Farbthema und Spracheinstellung werden
in diesem Browser gespeichert.

Verwenden Sie die Steuerelemente oben rechts, um zwischen **hellem und dunklem Modus** zu wechseln
und Ihre **Sprache** auszuwählen.

## Einen Scan starten

Klicken Sie auf **Neuer Scan** (oben links oder in der Seitenleiste). Wählen Sie das Scan-Ziel:

- **Repository** — eine HTTPS-Klon-URL, z. B. `https://github.com/owner/repo`.
- **Benutzer** oder **Organisation** — jedes Repository, das diesem Konto gehört. Aktivieren Sie
  *Geforkte Repositories einschließen*, um auch geforkte Repos zu scannen.

Der Scan wird im Hintergrund ausgeführt. Es öffnet sich die Detailansicht des Scans, in der
Fortschrittsbalken und Zähler aktualisiert werden, während die Repositories durchsucht werden.

## Ergebnisse auswerten

Die **Übersicht** zeigt die wichtigsten Kennzahlen: offene Funde, Anzahl kritischer Funde, wie viele
noch aktiv auf HEAD liegen, sowie Aufschlüsselungen nach Schweregrad, Anbieter und Triage-Status.
Jede Kachel und jeder Balken verlinkt direkt auf eine entsprechend gefilterte Funde-Liste.

**Analysen** bietet tiefere Einblicke — eine Zeitachse über die Einführung von Anmeldedaten sowie
Aufschlüsselungen nach Anwendungstyp und Erkennungsregel.

**Funde** ist die vollständige, filterbare Liste. Filtern Sie nach Schweregrad, Anbieter, Status
oder Freitext; der Filter ist in der URL enthalten, sodass gefilterte Ansichten geteilt oder als
Lesezeichen gespeichert werden können. Jede Zeile zeigt:

- den Schweregrad und den Anbieter, zu dem die Anmeldedaten gehören;
- eine maskierte Vorschau — das echte Geheimnis verlässt niemals die Scan-Engine;
- Datei und Zeilennummer mit direktem Link zur Datei auf GitHub;
- den Commit, der das Geheimnis eingeführt hat, samt Autor und Datum;
- **Exposition** — `auf HEAD` bedeutet, dass es sich noch im aktuellen Baum befindet; `Historie`
  bedeutet, dass es entfernt wurde, aber beim Klonen noch erreichbar ist;
- ein Badge **Aktiv — erneuern**, wenn die Live-Prüfung bestätigt hat, dass der Schlüssel noch funktioniert.

Verwenden Sie das Menü am Ende einer Zeile, um einen Fund bei der Bearbeitung als **Erneuert**,
**Fehlalarm** oder **Ignoriert** zu markieren.

## Repositories und nicht zugeordnete Muster

**Repositories** sortiert alle gescannten Repositories nach ihrer offenen Exposition, aufgeschlüsselt
nach Schweregrad. **Nicht zugeordnet** ist das Backlog von High-Entropy-Geheimnissen, die noch keiner
benannten Regel zugeordnet wurden, gruppiert nach Mustern — ein häufig auftretendes Muster ist ein
Anbieter, für den sich das Erstellen einer Regel lohnt.

## Live-Validierung

Wenn der Operator die Funktion für seine Ziele aktiviert hat, führt GitScanner pro Fund einen
schreibgeschützten Identitätsaufruf durch, um festzustellen, ob der Schlüssel noch gültig ist und zu
welchem Konto er gehört. Ein bestätigter aktiver Schlüssel sollte umgehend erneuert werden. Diese
Funktion ist standardmäßig deaktiviert und listet niemals Kontoinhalte auf.
