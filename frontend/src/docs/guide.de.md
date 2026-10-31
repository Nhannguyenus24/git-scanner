# Verwendung von GitScanner

GitScanner klont die gesamte Historie eines Repositories, liest jede eindeutige Dateiversion
(Blob) genau einmal und meldet die gefundenen Anmeldedaten — im aktuellen Arbeitsbaum und an
beliebiger Stelle in der Vergangenheit. Ein vor drei Jahren gelöschter Schlüssel ist für jeden,
der das Repository klont, weiterhin lesbar und bleibt daher ein Leak, bis er widerrufen wird.

Diese Anleitung führt Seite für Seite durch das Dashboard. Die Kurzfassung:

1. Prüfen Sie, ob der Punkt neben der API-Adresse in der oberen Leiste grün leuchtet.
2. Klicken Sie auf **Neuer Scan** und geben Sie ein Repository, einen Benutzer oder eine Organisation an.
3. Verfolgen Sie die Seite des Scans, bis er abgeschlossen ist.
4. Arbeiten Sie die **Funde** ab: Erneuern Sie, was aktiv ist, und markieren Sie, was kein Leak ist.
5. Informieren Sie die Eigentümer per Hinweis-Mail unter **Repositories**.

---

## Der Bildschirm

### Obere Leiste

- **API-Adresse** — der Server, von dem alles auf dem Bildschirm stammt. Der Punkt daneben
  ist grau während der Prüfung, grün, wenn die API antwortet, und rot, wenn sie nicht
  erreichbar ist. Tragen Sie Ihren eigenen Host ein, falls Sie nicht den Standard
  `http://localhost:8080` verwenden; er wird in diesem Browser gespeichert.
- **Befehlsfeld** — öffnet die Befehlspalette (siehe unten).
- **Hilfe** — diese Seite.
- **Hell / Dunkel** — wechselt zwischen hellem und dunklem Modus.
- **Sprache** — English, Tiếng Việt, Italiano, 中文, Español, Deutsch und
  Français. Alles, einschließlich dieser Anleitung, wechselt mit.

### Seitenleiste

Jede Seite ist nur einen Klick entfernt. Drei Einträge tragen eine Live-Zahl: **Funde**
(offene Funde), **Scans** (noch laufende Scans) und **Nicht zugeordnet** (Muster, die auf
eine Anbieterregel warten). Mit dem Pfeil am unteren Ende oder mit `Ctrl+[` (`⌘[` auf einem
Mac) klappen Sie die Seitenleiste auf reine Symbole ein. Auf einem Telefon wird die
Seitenleiste zu einer unteren Leiste; die seltener genutzten Seiten liegen unter **Mehr**.

### Tastatur

- `Ctrl+K` / `⌘K` — die Befehlspalette von überall öffnen oder schließen.
- `/` — die Befehlspalette öffnen.
- `N` — einen neuen Scan starten.
- `Ctrl+[` / `⌘[` — die Seitenleiste ein- oder ausklappen.
- `Esc` — einen Dialog, eine Seitenleiste oder die Palette schließen.

`N` und `/` werden ignoriert, während Sie in ein Textfeld tippen, sodass sie nie einen
Tastendruck verschlucken, der für einen Filter gedacht war.

### Befehlspalette

Ein Feld für alles, was das Dashboard über die Tastatur kann. Tippen Sie, um die Liste
einzugrenzen, bewegen Sie sich mit `↑` `↓` (oder `Ctrl+N` / `Ctrl+P`) und führen Sie mit
`Enter` aus. Angeboten werden:

- **Aktionen** — einen neuen Scan starten; die aktuelle Ansicht exportieren.
- **Gehe zu** — jede Seite.
- **Filter** — die Fund-Vorlagen und Ihre gespeicherten Filter, in einem Schritt angewendet.
- **Darstellung** — hell oder dunkel, dem System folgen, jedes Thema, Dichte,
  Bewegung, Seitenleiste und Zurücksetzen.
- **Sprache** — die Sprache der Oberfläche wechseln.

Alles andere, was Sie eingeben, wird zu **Funde nach „…“ durchsuchen** und öffnet die Funde
mit diesem Text als Suchfilter. Die Palette speichert bewusst keinen Verlauf: Ein Suchfeld,
das sich etwas merkt, ist ein Ort, an dem eingefügte Anmeldedaten liegen bleiben könnten.

Ist auf dem Server ein Claude-API-Schlüssel konfiguriert, bietet eine Eingabe aus mehr als
einem Wort zusätzlich **Claude fragen: „…“** an. Daraus wird ein Satz wie *kritische
AWS-Schlüssel noch auf HEAD in der Organisation acme* in die Filter der Funde übersetzt, die
Funde werden damit geöffnet, und Sie sehen, wie die Frage verstanden wurde, sodass Sie sie
korrigieren können. An Claude gehen nur die Frage und die Liste der Filterwerte — niemals ein
Fund —, und eine Frage, die nach Anmeldedaten aussieht, wird abgelehnt, bevor irgendetwas
gesendet wird.

### Einführung

Beim ersten Öffnen bietet das Dashboard eine Einführung in sieben Schritten an. Überspringen
Sie sie jederzeit; unter **Einstellungen → Einführung** können Sie sie später erneut ansehen.

---

## Einen Scan starten

Klicken Sie auf **Neuer Scan** (Seitenleiste, Übersicht- oder Scans-Seite, oder `N`). Wählen
Sie ein Ziel:

- **Repository** — eine HTTPS-Klon-URL, zum Beispiel
  `https://github.com/owner/repo`. SSH-URLs und URLs mit einem Benutzernamen oder Token
  darin werden abgelehnt.
- **Benutzer** — jedes Repository, das einem GitHub-Benutzer gehört. Ein Benutzername besteht
  aus Buchstaben, Ziffern und Bindestrichen, 1–39 Zeichen.
- **Organisation** — jedes Repository, das einer GitHub-Organisation gehört.
- **Zufällig** — eine Zufallsstichprobe öffentlicher Repositories, nur für Erkennungstests.
  Geben Sie die Anzahl ein, oder aktivieren Sie **Endlos ausführen**, um weiter Stichproben zu
  ziehen, bis Sie pausieren oder abbrechen.

Bei Benutzer, Organisation und Zufällig aktivieren Sie **Geforkte Repositories einschließen**,
um auch Forks zu scannen; standardmäßig werden sie übersprungen, weil ein Fork größtenteils
sein Eltern-Repository wiederholt.

Klicken Sie auf **Scan starten** oder drücken Sie `Enter`. Der Scan wird im Hintergrund
eingereiht, und das Dashboard öffnet seine Detailseite.

---

## Einen Scan verfolgen

### Scan-Liste

**Scans** listet jeden von der API akzeptierten Auftrag auf, neueste zuerst. Wechseln Sie
zwischen **Alle**, **Aktiv** und **Abgeschlossen**. Jede Zeile zeigt den Status, das Ziel,
den Fortschritt, gelesene Blobs, neue Funde, den Startzeitpunkt und die Laufzeit. Klicken Sie
auf eine Zeile, um sie zu öffnen; mit den Symbolen am Ende scannen Sie einen beendeten Auftrag
erneut oder löschen ihn.

Ein Auftrag befindet sich in einem von sechs Zuständen: **in Warteschlange**, **läuft**,
**pausiert**, **abgeschlossen**, **fehlgeschlagen** oder **abgebrochen**.

### Scan-Details

Die Kopfzeile nennt das Ziel, die Art des Scans, wann er eingereicht wurde und wie lange er
schon läuft. Die Schaltflächen ändern sich mit dem Zustand:

- **Pausieren** (läuft) — die Worker nehmen keine neuen Repositories mehr auf; bereits
  Gefundenes geht nicht verloren. **Fortsetzen** macht dort weiter, wo er angehalten hat.
- **Abbrechen** (jeder nicht beendete Auftrag) — stoppt den Auftrag endgültig.
- **Erneut scannen** (beendet) — reiht dasselbe Ziel erneut als neuen Auftrag ein.
- **Löschen** (beendet) — entfernt den Auftrag **und alle von ihm aufgezeichneten Funde**.
  Dies kann nicht rückgängig gemacht werden.

Unterhalb der Kopfzeile:

- **Zähler** — erledigte Repositories von der Gesamtzahl, gescannte Blobs und neue
  Funde (erstmals in diesem Auftrag gesehen).
- **Fortschritt und Restzeit** — eine Schätzung erscheint, sobald genug Daten vorliegen;
  bis dahin sind die Zähler maßgeblich.
- **Phasen** — Repositories ermitteln → Repositories scannen (klonen, Historie durchlaufen,
  erkennen) → Anbieter zuordnen → Auswertung abschließen.
- **Pipeline** — ein Punkt pro Repository, in dem Schritt, den sein Worker zuletzt gemeldet
  hat (klonen, Historie lesen, zuordnen, prüfen, speichern), und er rückt weiter, sobald der
  nächste Schritt gemeldet wird; fertige Repositories fallen in den Behälter rechts. Orange
  heißt bisher Kandidaten-Treffer, rot ein fehlgeschlagener Versuch, grau übersprungen.
- **Live-Aktivität** — während des Scans von den Workern gestreamt.
- **Zusammenfassung** — wenn der Auftrag abgeschlossen ist: wie viele Funde in wie vielen
  Repositories, wie viele davon hoch oder kritisch sind, wie viele noch vom Standard-Branch
  aus erreichbar sind und welche Repositories betroffen sind.
- **Durch diesen Scan aufgezeichnete Funde** — die Liste selbst, mit einem Link, um sie
  in den Funden zu öffnen.

### Live-Aktivität

Das Live-Panel zeigt, was jeder Worker gerade tut. Sein Status lautet
**verbinde…**, **live**, **verbinde neu…**, **beendet** oder **nicht verfügbar**; streamt
der Server nicht, werden die Zähler darüber trotzdem alle paar Sekunden aktualisiert.

- **Erkennung** — bei Benutzer- und Organisations-Scans die Auflistung der Repositories des
  Kontos, Seite für Seite. *wartet auf das GitHub-Ratenlimit* ist eine Pause,
  kein Fehler.
- **In Arbeit** — jedes Repository und der Schritt, in dem es sich befindet: **klonen** (bare:
  nur Branches und Tags), **Historie lesen** (jede eindeutige Dateiversion, mit der laufenden
  Zahl der Kandidaten-Treffer), **zuordnen** (jeden Treffer bis zu dem Commit zurückverfolgen,
  der ihn eingeführt hat), **prüfen** (Live-Validierung, falls aktiviert) und
  **speichern**.
- **Gerade fertig** — eben abgeschlossene Repositories, mit ihren neuen Funden, gelesenen
  Blobs und benötigter Zeit.
- **Aktivitätsprotokoll** — der vollständige Ereignisstrom; **weitere anzeigen** klappt ihn auf.

Ein Repository, dessen Versuch fehlschlägt, wird automatisch wiederholt und zeigt *Versuch
2*, *Versuch 3*… Ein Repository, dessen letzter Versuch fehlschlägt, wird **übersprungen**;
der Rest des Auftrags läuft weiter.

### Warum ein Scan nichts Neues finden kann

Anmeldedaten werden einmal aufgezeichnet, und zwar vom ersten Scan, der sie gefunden hat. Ein
erneuter Scan desselben Repositories findet dieselben Schlüssel, zeichnet aber keine *neuen*
Funde auf — sie bleiben dem früheren Scan zugeordnet. Öffnen Sie die nach diesem Repository
gefilterten **Funde**, um alle zu sehen.

---

## Übersicht

Die Seite mit den wichtigsten Kennzahlen, alle zehn Sekunden aktualisiert (die Markierung
**Live** zeigt das Intervall).

- **Sicherheitslage** — ein Score von 100. Klicken Sie auf **Wie es berechnet wird**, um
  jeden Abzug zu sehen (siehe unten).
- **Kennzahlen** — offene Funde, kritisch, aktiv auf HEAD, exponierte
  Repositories, aktive Scans, nicht zugeordnete Muster.
- **Offen nach Schweregrad**, **Offen nach Anbieter**, **Triage** — Aufschlüsselungen. Jede
  Kachel und jeder Balken verlinkt auf eine gefilterte Funde-Liste.
- **Letzte Aktivität** — abgeschlossene oder fehlgeschlagene Scans, neue Funde, gesendete
  Hinweise.
- **Kürzliche Scans** — die neuesten Aufträge und ihr Fortschritt.

### Layouts

**Layout** wechselt zwischen vier Anordnungen: **Alles**, **Tägliche Triage**
(Zahlen und Aufschlüsselungen, ohne Score), **Management-Überblick** (Score,
Kennzahlen, was sich geändert hat) und **Sicherheitsbetrieb** (Aktivität und
laufende Scans zuerst). Mit **Anpassen** verschieben Sie ein Panel mit den Pfeilen nach oben
oder unten, blenden es mit dem Auge aus und holen ein ausgeblendetes aus der Liste darunter
zurück. Layouts werden in diesem Browser gespeichert.

### Wie der Score der Sicherheitslage berechnet wird

Der Score beginnt bei 100 und verliert Punkte pro Faktor. Jeder Abzug sättigt, sodass die
ersten Funde am meisten kosten und kein einzelner Faktor ausufern kann:

- **Kritische Funde** — bis zu 30 Punkte.
- **Von HEAD erreichbar** — bis zu 25 Punkte. Das schärfste Signal: Jeder, der heute den
  Standard-Branch klont, erhält die Anmeldedaten.
- **Hohe Funde** — bis zu 15 Punkte.
- **Betroffene Repositories** — bis zu 15 Punkte, nach dem Anteil der gescannten
  Repositories mit einem offenen Fund.
- **Ungelöster Anteil** — bis zu 10 Punkte, nach dem Anteil der noch offenen Funde.
  Ein geschlossener Fund kostet nichts.
- **Eigentümer nicht benachrichtigt** — bis zu 5 Punkte, für exponierte Repositories ohne
  gesendeten Hinweis.

Ab 85 lautet die Bewertung **Stabil**, 65–84 **Verbesserungswürdig**, 40–64 **Exponiert**,
unter 40 **Kritisch**. Wurde noch nichts gescannt, gibt es gar keinen Score statt einer
irreführenden 100. Der Score wird im Browser aus den datenbankweiten Zählern des Servers
berechnet, nie aus einer einzelnen Seite von Funden.

---

## Funde

Die vollständige Liste aller aufgezeichneten Anmeldedaten, 50 Zeilen pro Seite.

### Filtern

- **Suche** — Pfad, Regel, Anbieter oder Repository.
- **Schweregrad**, **Anbieter**, **Status** (Offen, Erneuert, Fehlalarm,
  Ignoriert).
- **Exposition** — auf HEAD, nur in der Historie oder als aktiv bestätigt.
- **Reihenfolge** — Standard, zuletzt erkannt, ältester Commit oder Schweregrad.
- **Ein Repository** / Scan — erscheint als Chip, wenn Sie von einem Repository oder
  einem Scan kommen; entfernen Sie ihn, um alles zu sehen.

Der Filter steht in der URL, sodass eine gefilterte Ansicht als Lesezeichen gespeichert,
geteilt oder nach einem Neuladen wieder geöffnet werden kann. **Filter zurücksetzen** setzt
ihn zurück.

Exposition und Reihenfolge werden im Browser berechnet. Ist eines davon aktiv, lädt die Seite
bis zu 500 Funde vom Server und verfeinert diese; gab es mehr, steht das über der Tabelle,
statt eine irreführende Gesamtzahl anzuzeigen.

### Vorlagen und gespeicherte Filter

Das Menü **Filter** bietet fertige Ansichten:

- **Kritisch, noch offen** — diese zuerst erneuern.
- **Von HEAD erreichbar** — von jedem lesbar, der heute klont.
- **Als aktiv bestätigt** — die Prüfung erhielt eine gültige Antwort.
- **Nicht zugeordnet** — keine Anbieterregel hat sie beansprucht.
- **Kürzlich entdeckt** — neueste Funde zuerst.
- **Am längsten ungelöst** — ältester Commit zuerst.

**Aktuellen Filter speichern** legt den eingestellten Filter unter einem Namen ab; er kann im
selben Menü umbenannt oder gelöscht werden und wird auch in der Befehlspalette angeboten.
Gespeicherte Filter liegen nur in diesem Browser und enthalten nur Filterfelder —
niemals einen Suchtext, der wie ein eingefügtes Geheimnis aussieht.

### Eine Zeile lesen

- **Schweregrad** und der **Anbieter**, zu dem die Anmeldedaten gehören (oder
  *nicht zugeordnet*).
- **Anmeldedaten** — eine maskierte Vorschau. Der echte Wert verlässt die Engine nie,
  es sei denn, Sie haben das Demaskieren ausdrücklich eingeschaltet (siehe Einstellungen).
- **Repository** und **Ort** — Datei und Zeile, verlinkt auf die Datei auf
  GitHub.
- **Exposition** — `auf HEAD` bedeutet, dass es sich noch im aktuellen Baum befindet; `Historie`
  bedeutet, dass es entfernt wurde, aber durch Klonen noch erreichbar ist. *Commit unbekannt —
  erneut scannen zur Zuordnung* bedeutet, dass der einführende Commit nicht ermittelt wurde;
  ein erneuter Scan ergänzt ihn.
- **Aktiv — erneuern** — die Live-Validierung hat bestätigt, dass der Schlüssel noch funktioniert.
- **Status** und Zeitpunkt **Erkannt**.

Mit dem Menü am Ende einer Zeile markieren Sie einen Fund als **Offen**, **Erneuert**,
**Fehlalarm** oder **Ignoriert**.

### Fund-Details

Klicken Sie auf eine Zeile, um die Detail-Seitenleiste zu öffnen. Ihre Adresse wird in die URL
übernommen, sodass **Link kopieren** einem Kollegen dieselbe Ansicht gibt; **Referenz kopieren**
kopiert eine kurze Referenz für ein Ticket. Die Seitenleiste zeigt:

- **Zugangsdaten** — maskierte Vorschau, Kategorie und Gültigkeit. Wurde der Wert für einen
  unmaskierten Eigentümer aufbewahrt und ist die Maskierung aus, zeigt **Anzeigen** ihn an,
  markiert als **KLARTEXT**.
- **Kontingent & Limits** — was der Anbieter bei der Aktivitätsprüfung gemeldet hat:
  verbleibende Anfragen oder Tokens, Berechtigungen, Land, Währung. Eine Momentaufnahme zum
  Zeitpunkt des Scans.
- **Warum das erkannt wurde** — die Regel, der zugeordnete Anbieter, die Länge und der
  Zeichensatz, eine Variable oder ein Hostname in der Nähe sowie die Exposition.
  Die Erklärung nutzt nur Form, Länge und Kontext, niemals den Wert.
- **Fundort** — Datei, Commit, Autor und Commit-Datum, mit einem Link zur Zeile auf
  GitHub.
- **Verlauf** — committet, erkannt, geprüft und Triage-Entscheidungen.
- **Was zu tun ist** — die Behebungsschritte der Reihe nach: beim Anbieter widerrufen und neu
  ausstellen, dessen Audit-Log prüfen, den Wert aus HEAD entfernen, entscheiden, ob die
  Historie umgeschrieben wird, und eine Pre-Commit-Geheimnisprüfung hinzufügen.
- **Gleiches Repository** und **Gleicher Anbieter** — verwandte Funde, einen Klick entfernt.
- **Metadaten** — alles Weitere, was die Engine aufgezeichnet hat.

### Massen-Triage

Aktivieren Sie die Kästchen am Anfang der Zeilen oder das Kästchen in der Kopfzeile, um alle
Zeilen der Seite auszuwählen. Unten erscheint eine Leiste mit der Anzahl und den vier Status.
**Erneuert** und **Fehlalarm** verlangen zuerst eine Bestätigung, denn beide sind eine Aussage
über die Außenwelt — dass der Schlüssel widerrufen wurde, dass der Treffer falsch war — und in
der Masse ist es genau diese Aussage, die ein echtes Leak stillschweigend schließen kann. Offen
und Ignoriert gelten sofort. **Auswahl exportieren** exportiert nur die markierten Zeilen.

### Exportieren

**Exportieren** schreibt die aktuell geladenen Funde, wobei die Filter, die sie erzeugt haben,
in der Datei festgehalten werden:

- **Funde als CSV** — eine Zeile pro Fund, für eine Tabelle.
- **Funde als JSON** — strukturiert, für ein Skript.
- **Vorfallbericht** — Markdown, nach Repository gruppiert.
- **Management-Zusammenfassung** — eine Seite, ohne Tabelle.

Exporte enthalten nur maskierte Vorschauen; die Anmeldedaten im Klartext werden in keinem
Format jemals in eine Datei geschrieben.

---

## Repositories

Jedes gescannte Repository, sortiert danach, wie schwer seine offene Exposition wiegt.

- **Nach Besitzer/Name filtern** und **Nur mit offenen Funden**.
- **Sortieren nach** Risiko, offenen Funden, zuletzt gescannt oder Name.
- Spalten: Typ, offene Anzahl je Schweregrad, Offen / Gesamt, **Risiko**, **Nächster
  Schritt**, **Benachrichtigt** und zuletzt gescannt.
- Klicken Sie auf ein Repository, um seine Funde zu öffnen.

**Risiko** reicht von 0 (nichts offen) bis 100. Offene Funde werden nach Schweregrad
gewichtet — ein kritischer zählt zehnmal so viel wie ein niedriger — und die Summe sättigt,
sodass ein Repository mit einem einzigen kritischen Fund bereits weit oben steht. Stufen:
Niedrig, Mittel (25+), Hoch (50+), Kritisch (75+). Fahren Sie mit der Maus darüber, um die
zugrunde liegenden Zahlen zu sehen.

**Nächster Schritt** nennt das eine, was sich zu tun lohnt: *Kritische Schlüssel rotieren*,
*Eigentümer informieren*, *Meldung in Warteschlange*, *Meldung fehlgeschlagen – erneut
versuchen*, *Verbleibende Funde prüfen* oder *Nichts offen*.

### Den Eigentümer informieren

Der Chip **Hinweis-Mails** oben zeigt, ob der Mailversand eingeschaltet ist; er verlinkt auf
die Einstellungen. Solange Mails eingeschaltet sind, reiht **Benachrichtigen** in einer Zeile
einen Hinweis an den Repository-Eigentümer ein, der dessen Funde mit maskierten Vorschauen
auflistet. Die Spalte **Benachrichtigt** zeigt dann *wird gesendet*, *gesendet*,
*fehlgeschlagen* oder *abgemeldet*.

### Zeitmaschine

Das Uhrsymbol in einer Zeile (oder der Chip **Zeitmaschine** in den auf ein Repository
gefilterten Funden) spielt die Exposition dieses Repositorys Commit für Commit ab. Jede Datei
mit Anmeldedaten bekommt eine Bahn; eine Raute markiert den Commit, der den jeweiligen
Schlüssel eingeführt hat, und von dort führt eine Linie bis heute — durchgezogen, solange der
Schlüssel noch auf HEAD liegt, verblassend, sobald er entfernt wurde.

Ziehen Sie den Schieberegler, gehen Sie mit den Schaltflächen Zurück und Weiter schrittweise
vor oder drücken Sie Abspielen, um vom ersten Leck an vorwärts zu laufen. Bei jedem Schritt
zeigt die Seite, wie viele Anmeldedaten bis dahin committet waren, wie viele heute noch auf
HEAD liegen und dass alle weiterhin klonbar sind. Darunter stehen die Commits, die
Anmeldedaten eingeführt haben, neueste zuerst, mit Autor, Datum und einem Link zu jedem Fund.

Der Scan weiß, wann ein Schlüssel committet wurde, nicht aber, wann er gelöscht wurde; die
Linie eines entfernten Schlüssels verblasst deshalb, statt an einem erfundenen Datum zu
enden. Funde ohne Commit-Datum werden unter der Liste gezählt; ein erneuter Scan ordnet sie
ein.

---

## Schlüsselverbreitung

Anmeldedaten, die in mehr als einem Repository gefunden wurden. Funde werden einmal pro
Repository erfasst, daher zeigt sich dasselbe Geheimnis an mehreren Stellen nur hier, wo die
Kopien über ihren Fingerabdruck verbunden werden.

- **Geteilte Schlüssel**, **Über Eigentümer hinweg** (Schlüssel in Repositorys verschiedener
  Eigentümer), **Offen auf HEAD** und betroffene **Repos / Eigentümer**. Gibt es mehr geteilte
  Schlüssel, als der Graph zeichnet, nennt die erste Karte, wie viele gezeichnet sind.
- **Gefunden in** — Schlüssel in 2, 3, 5 oder 10 und mehr Repositories anzeigen.
- **Der Graph** — eine Raute pro Schlüssel, nach Schweregrad gefärbt und nach Verbreitung
  skaliert; ein Kreis pro Repository; ein Quadrat für jeden Eigentümer mit zwei oder mehr
  davon. Eine rote Linie ist eine Kopie, die noch auf HEAD liegt, eine gestrichelte eine
  bereits gesichtete Kopie. Mit der Maus heben Sie die Nachbarschaft eines Schlüssels hervor.
- **Die Liste** daneben enthält dieselben Schlüssel für Tastatur und Screenreader. Wird ein
  Schlüssel in einer der beiden gewählt, erscheinen seine Repositories, jedes mit seinem Fund
  verlinkt.

Eine Rotation behebt jede Kopie, daher kann die Detailansicht eines Schlüssels nach einer
Bestätigung auf einmal **alle Kopien markieren**, als *Erneuert* oder *Fehlalarm*. Ein
Wert, der in die Repositories vieler voneinander unabhängiger Eigentümer kopiert wurde, ist
meist Boilerplate, die der generische Durchlauf für ein Geheimnis gehalten hat —
Framework-Klassennamen, Beispielschlüssel aus einem Tutorial —, und diese Ansicht ist der
schnellste Weg, sie zu erkennen.

---

## Nicht zugeordnet

Das Backlog von High-Entropy-Geheimnissen, die keine benannte Anbieterregel beansprucht,
gruppiert nach Form. Jede Zeile zeigt einen **vermuteten Namen** (aus der nächstgelegenen
Variable oder dem nächsten Schlüssel — manchmal nur ein Wort in der Nähe), ein maskiertes
**Muster**, die **Musterform** (Länge und Zeichensatz), den **Kontext**, in dem es gefunden
wurde, wie oft es **gesehen** wurde, seinen Status und wann es zuletzt gesehen wurde. Eine
Form, die immer wieder auftaucht, ist ein Anbieter, für den sich eine Regel lohnt. Solche
Funde erscheinen als *nicht zugeordnet*, bis eine Regel sie beansprucht; sobald das geschieht,
ordnet der nächste Scan sie neu zu.

---

## Anbieter

Jeder Anbieter, den GitScanner erkennen und zuordnen kann, gruppiert nach Kategorie (KI,
Cloud, Daten, Zahlungen, Messaging, DevOps, Entwicklerwerkzeuge, Sonstiges), mit der Anzahl
der Regeln für jeden. Klicken Sie auf einen Anbieter, um Folgendes zu sehen:

- **Warum dieser Schweregrad** — kritisch bedeutet volle Kontrolle; hoch ist breiter
  API-Zugriff; mittel ist enger oder auf einen Host beschränkter Zugriff; niedrig identifiziert
  ein Konto, kann aber nicht in seinem Namen handeln.
- **Was ein Leak jemandem ermöglicht** — die Auswirkung für diese Kategorie.
- **Wie die Schlüssel aussehen und wie sie erkannt werden**, eine Karte pro Regel:
  - *eigenes Präfix* — der Wert beginnt mit dem Präfix des Anbieters, was für sich allein
    schon ein Beweis ist;
  - *Einstellungsname* — ein Wert ohne eigene Form, der nur gemeldet wird, wenn er einer
    nach dem Anbieter benannten Einstellung zugewiesen ist;
  - *Verbindungs-URL* — eine URL mit den Anmeldedaten darin; eine, die nur auf localhost
    oder einen Compose-Dienst zeigt, gilt als Entwicklungskonfiguration und wird
    verworfen;
  - *PEM-Block* — ein privater Schlüssel von BEGIN bis END;
  - *nur Aufbau* — feste Längen, Trennzeichen oder Markierungen, bewusst eng
    gehalten.
  Jede Karte kann den Aufbau des Schlüssels, das Muster, den Text, den die Datei enthalten
  muss, und ein Wort, das in der Nähe stehen muss, zeigen.
- **Nach einem Treffer** — Platzhalter (`your-…-key`, `xxxx`, `changeme`),
  Dokumentations- und Sandbox-Schlüssel sowie Test-Fixtures werden verworfen; was übrig bleibt,
  wird nur als Fingerabdruck und maskierte Vorschau gespeichert.

Ein Anbieter zeigt seine schwerwiegendste Regel. Bei jedem Fund steigt der Schweregrad um eine
Stufe, wenn das Leak noch von HEAD erreichbar ist, und um eine weitere bei Cloud- und
Zahlungsanbietern, bis maximal kritisch.

---

## Analysen

Ein Blick auf alles, was bisher gescannt wurde:

- **Funde** und **Repositories** — Gesamtzahlen, offen, exponiert von gescannt.
- **Leck-Kalender** — ein Quadrat pro Tag, jeweils ein Jahr, schattiert nach der Zahl der an
  diesem Tag committeten Anmeldedaten und umrandet, wo eine kritisch war. Das Jahr wählen Sie
  oben rechts. Die Tage sind UTC: Die Zeitzone des Autors wird nicht erfasst, ein Quadrat kann
  also dessen vorherigen oder nächsten Tag sein.
- **Zeitachse der Leakeinführung** — Funde nach dem Monat, in dem die Anmeldedaten
  committet wurden.
- **Offen nach Schweregrad**, **Offen nach Anbieter** (benannte Anbieter plus die nicht
  zugeordnete Sammelkategorie), **Nach Anwendungstyp**, **Triage-Status**.
- **Erkennungsregeln** — welche Regeln am häufigsten anschlagen.
- **Validierung aktiver Anmeldedaten** — aktiv, widerrufen, kein Prober, Prüfung fehlgeschlagen.
- **Hinweis-Mails** — kontaktierte Repositories und exponierte, die noch nicht
  benachrichtigt wurden.
- **Am stärksten betroffene Repositories** — nach demselben Risiko-Score wie bei Repositories.
- **Am längsten ungelöst** — offene Funde nach Commit-Datum.

Alle Zahlen gelten für den gesamten Zeitraum; es gibt keinen Datumsbereich, weil die API
keinen anbietet. Die Filter oben steuern die Funde-Links, die diese Seite öffnet.

---

## Hinweis-Mails (Benachrichtigungen)

Jeder an Repository-Eigentümer gesendete Hinweis und sein Ergebnis: Repository,
Empfänger, Status (*wird gesendet*, *gesendet*, *fehlgeschlagen*, *abgemeldet*), Anzahl der
Funde, wann er angefordert und wann er gesendet wurde. Suchen Sie nach Repository oder
Empfänger.

- **Mail ansehen** — die Nachricht als HTML und als reiner Text, aus den aktuellen Funden
  neu erzeugt.
- **Erneut senden** — reiht sie erneut ein.

---

## Einstellungen

### Darstellung

Nur in diesem Browser gespeichert.

- **Thema** — Papier (gealtertes Papier, Petrol und Ochsenblut), Mitternacht (fast schwarzes
  Terminal, Phosphorgrün), Ozean (Marineblau, Cyan und Blau), Signal (Graphit, nur Alarme
  tragen Farbe), Minimal (schlichtes Unternehmens-Design).
- **Hell oder dunkel** — Hell, Dunkel oder System, um Ihrem Betriebssystem zu folgen.
- **Dichte** — Kompakt zeigt mehr Zeilen pro Bildschirm.
- **Bewegung** — Reduziert schaltet Animationen ab.
- **Mit eingeklappter Seitenleiste starten**.
- **Darstellung zurücksetzen** — zurück zu den Standardwerten.

### Einführung

**Einführung erneut ansehen** öffnet die Einführungstour noch einmal.

### Browser-Benachrichtigungen

Ist dies eingeschaltet und die Berechtigung des Browsers erteilt, benachrichtigt Sie ein
offenes Dashboard — ein Hintergrund-Tab genügt —, wenn ein von ihm verfolgter Scan endet und
wenn weitere Schlüssel als aktiv bestätigt werden; ein Klick auf die Benachrichtigung öffnet
den Scan oder die aktiven Schlüssel. Während Sie auf das Dashboard schauen, kommt dieselbe
Nachricht stattdessen als Meldung in der Seite. Sind alle Dashboard-Tabs geschlossen, wird
nichts gesendet. Nur in diesem Browser gespeichert.

### Das Dashboard installieren

Das Dashboard lässt sich über die Adressleiste des Browsers als App installieren (oder auf
einem Telefon mit *Zum Startbildschirm hinzufügen*). Es öffnet sich dann in einem eigenen
Fenster, und seine Oberfläche lädt auch ohne Netzwerk; die Daten kommen weiterhin von der API
und werden nie im Browser-Cache gespeichert.

### Hinweis-Mails

Der Hauptschalter. Ist er an, sendet Benachrichtigen einen Hinweis an den Repository-Eigentümer.
Ist er aus, wird nichts gesendet, und bereits Eingereihtes wird verworfen. Einstellungen pro
Schweregrad oder als Sammelmail gibt es noch nicht.

### Sicherheitsmaskierung und unmaskierte Eigentümer

**An** (Standard): Alle Anmeldedaten werden maskiert angezeigt. **Aus**: Anmeldedaten, die in
Repositories der unter **Unmaskierte Eigentümer** aufgeführten Eigentümer gefunden wurden,
werden zu Testzwecken vollständig angezeigt; alle anderen bleiben maskiert, und Mails und
Exporte sind immer maskiert.

- Fügen Sie nur GitHub-Benutzer oder -Organisationen hinzu, die Ihnen gehören.
- Werte werden ab dem nächsten Scan verschlüsselt aufbewahrt — scannen Sie die Repositories
  eines Eigentümers erneut, um bestehende Funde unmaskiert zu sehen.
- Das Entfernen eines Eigentümers löscht alles, was für ihn aufbewahrt wurde.
- Hat der Server keinen `SECRET_SEAL_KEY`, ist das Demaskieren nicht verfügbar und es wird
  nie etwas aufbewahrt. Erzeugen Sie einen mit `openssl rand -hex 32`, tragen Sie ihn in
  `.env.local` ein und starten Sie neu.

Solange die Maskierung aus ist, bleibt eine rote Warnung auf der Seite: Die vollständigen
Anmeldedaten der aufgeführten Eigentümer sind für jeden sichtbar, der dieses Dashboard öffnen
kann.

---

## Live-Validierung

Wenn der Operator sie für seine eigenen Ziele einschaltet (`PROBE_SECRETS`), führt
GitScanner pro Fund einen schreibgeschützten Identitätsaufruf durch, um festzustellen, ob der
Schlüssel noch funktioniert und zu welchem Konto er gehört:

- **Aktiv — erneuern** — der Anbieter hat ihn akzeptiert. Erneuern Sie diesen zuerst.
- **Widerrufen** — der Anbieter hat ihn abgelehnt; bereits erneuert oder widerrufen.
- **Kein Prober** — für diesen Anbieter gibt es keine Prüfung.
- **Prüfung fehlgeschlagen** — Ratenbegrenzung oder Netzwerk; versuchen Sie später einen
  erneuten Scan.

Sie ist standardmäßig deaktiviert, listet niemals Kontoinhalte auf, und ihre Werte zu
Kontingent und Limits erscheinen in der Detail-Seitenleiste des Fundes.

---

## Schweregrad

- **Kritisch** — volle Kontrolle: Geld bewegen, das Konto übernehmen, alles lesen, was es
  schützt.
- **Hoch** — breiter API-Zugriff im Namen des Kontos.
- **Mittel** — enger oder auf einen Host beschränkter Zugriff: ein Kanal, ein Webhook, ein
  Server, der zusätzlich erreichbar sein muss.
- **Niedrig** — identifiziert ein Konto, kann aber nicht in seinem Namen handeln, etwa ein
  Testmodus-Schlüssel. Trotzdem eine Erneuerung wert.

---

## Fehlerbehebung

- **„API konnte nicht erreicht werden“** — prüfen Sie die Adresse in der oberen Leiste und ob
  der Stack läuft; der Punkt wird grün, sobald die API antwortet.
- **Ein Scan steht bei 0 %** — öffnen Sie ihn und sehen Sie sich die Live-Aktivität an.
  *wartet auf das GitHub-Ratenlimit* löst sich von selbst; gar keine Aktivität bedeutet, dass
  die Worker nicht laufen.
- **Ein erneuter Scan hat nichts Neues gefunden** — die Funde gehören zum früheren Scan;
  filtern Sie die Funde stattdessen nach dem Repository.
- **Commit unbekannt** — scannen Sie das Repository erneut, um ihn zuzuordnen.
- **Benachrichtigen ist ausgegraut** — schalten Sie Hinweis-Mails in den Einstellungen ein.
- **Anzeigen fehlt in der Seitenleiste** — die Maskierung ist an, der Eigentümer steht nicht
  auf der Liste der unmaskierten Eigentümer, der Server hat keinen `SECRET_SEAL_KEY`, oder das
  Repository wurde nach dem Hinzufügen des Eigentümers nicht erneut gescannt.
- **Die Liste meldet, dass nur 500 Funde verfeinert wurden** — grenzen Sie die
  serverseitigen Filter (Schweregrad, Anbieter, Status, Repository) ein, damit das Fenster
  alles abdeckt.
- **Kein „Claude fragen“ in der Palette** — der Server hat keinen `ANTHROPIC_API_KEY`, oder
  die Eingabe ist nur ein Wort (das ist eine normale Suche).
- **Claude fragen hat die Frage abgelehnt** — sie enthielt etwas, das wie Anmeldedaten
  aussieht; formulieren Sie sie ohne den Wert neu.
- **Keine Browser-Benachrichtigungen** — prüfen Sie die Berechtigung für diese Seite im
  Browser; eine blockierte Seite kann von der Seite aus nicht erneut gefragt werden.
- **Veraltete Ergebnisse** — ein gelöschter Auftrag oder gelöschte Daten entfernen die
  zugehörigen Funde; ein erneuter Scan bringt sie zurück.

GitScanner ist nur für autorisierte Sicherheitstests bestimmt. Scannen Sie, was Ihnen gehört
oder was Sie testen dürfen, und informieren Sie die Eigentümer über Ihre Funde.
