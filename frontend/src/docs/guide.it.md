# Utilizzo di GitScanner

GitScanner clona l'intera cronologia di un repository, legge una sola volta ogni
versione unica di file (blob) e segnala le credenziali che trova — nell'albero
corrente e in qualsiasi punto del passato. Una chiave cancellata tre anni fa è
ancora leggibile da chiunque cloni il repository, quindi resta una fuga di dati
finché non viene revocata.

Questa guida percorre la dashboard pagina per pagina. In breve:

1. Verifica che il punto accanto all'indirizzo dell'API nella barra superiore sia verde.
2. Premi **Nuova scansione** e indica un repository, un utente o un'organizzazione.
3. Segui la pagina della scansione finché non termina.
4. Lavora sui **Rilevamenti**: ruota ciò che è attivo, contrassegna ciò che non è una fuga.
5. Avvisa i proprietari con un'email di segnalazione da **Repository**.

---

## La schermata

### Barra superiore

- **Indirizzo API** — il server da cui proviene tutto ciò che vedi. Il punto
  accanto è grigio durante la verifica, verde quando l'API risponde e rosso
  quando non è raggiungibile. Inserisci il tuo host se non usi il valore
  predefinito `http://localhost:8080`; viene memorizzato in questo browser.
- **Comandi** — apre la palette dei comandi (vedi sotto).
- **Guida** — questa pagina.
- **Chiaro / scuro** — alterna tra carta chiara e carta scura.
- **Lingua** — English, Tiếng Việt, Italiano, 中文, Español, Deutsch e
  Français. Tutto, compresa questa guida, cambia di conseguenza.

### Barra laterale

Ogni pagina è a un clic di distanza. Tre voci mostrano un conteggio in tempo
reale: **Rilevamenti** (rilevamenti aperti), **Scansioni** (scansioni ancora in
corso) e **Non mappati** (pattern in attesa di una regola di provider). Riduci
la barra laterale alle sole icone con la freccia in fondo o con `Ctrl+[` (`⌘[`
su Mac). Su un telefono la barra laterale diventa una barra in basso; le pagine
meno usate si trovano sotto **Altro**.

### Tastiera

- `Ctrl+K` / `⌘K` — apre o chiude la palette dei comandi, da qualsiasi punto.
- `/` — apre la palette dei comandi.
- `N` — avvia una nuova scansione.
- `Ctrl+[` / `⌘[` — riduce o espande la barra laterale.
- `Esc` — chiude una finestra di dialogo, un pannello o la palette.

`N` e `/` vengono ignorati mentre scrivi in una casella di testo, quindi non
rubano mai un tasto destinato a un filtro.

### Palette dei comandi

Un'unica casella per tutto ciò che la dashboard può fare da tastiera. Digita per
restringere l'elenco, spostati con `↑` `↓` (o `Ctrl+N` / `Ctrl+P`), esegui con
`Enter`. Offre:

- **Azioni** — avvia una nuova scansione; esporta la vista corrente.
- **Vai a** — ogni pagina.
- **Filtri** — i filtri predefiniti dei rilevamenti e quelli che hai salvato,
  applicati in un solo passo.
- **Aspetto** — chiaro o scuro, segui il sistema, ogni tema, densità,
  animazioni, barra laterale e ripristino.
- **Lingua** — cambia la lingua dell'interfaccia.

Qualsiasi altro testo digitato diventa **Cerca rilevamenti per “…”**, che apre
Rilevamenti con quel testo come filtro di ricerca. La palette non conserva
volutamente alcuna cronologia: una casella di ricerca che ricorda è un posto in
cui una credenziale incollata potrebbe restare.

Quando il server ha una chiave API di Claude configurata, una ricerca di più di
una parola offre anche **Chiedi a Claude: “…”**. Traduce una frase come *chiavi
AWS critiche ancora su HEAD nell'organizzazione acme* nei filtri di Rilevamenti,
apre Rilevamenti con quei filtri e dice come ha interpretato la domanda, così
puoi correggerla. A Claude vengono inviati solo la domanda e l'elenco dei valori
di filtro — mai un rilevamento — e una domanda che sembra contenere una
credenziale viene rifiutata prima che venga inviato qualsiasi cosa.

### Introduzione

La prima volta che la dashboard si apre propone un'introduzione in sette passi.
Puoi saltarla quando vuoi e rivederla in seguito da **Impostazioni → Introduzione**.

---

## Avvio di una scansione

Premi **Nuova scansione** (barra laterale, pagina Panoramica o Scansioni, oppure
`N`). Scegli un obiettivo:

- **Repository** — un URL di clone HTTPS, ad esempio
  `https://github.com/owner/repo`. Gli URL SSH e quelli che contengono un nome
  utente o un token vengono rifiutati.
- **Utente** — ogni repository posseduto da un utente GitHub. Un login è fatto
  di lettere, cifre e trattini, da 1 a 39 caratteri.
- **Organizzazione** — ogni repository posseduto da un'organizzazione GitHub.
- **Casuale** — un campione casuale di repository pubblici, solo per testare il
  rilevamento. Indica quanti, oppure seleziona **Esegui continuamente** per
  continuare a campionare finché non metti in pausa o annulli.

Per Utente, Organizzazione e Casuale, seleziona **Includi repository derivati
(fork)** per scansionare anche i fork; sono esclusi per impostazione predefinita
perché un fork ripete per lo più il repository padre.

Premi **Avvia scansione** o `Enter`. La scansione viene messa in coda in
background e la dashboard ne apre la pagina di dettaglio.

---

## Seguire una scansione

### Elenco delle scansioni

**Scansioni** elenca ogni operazione accettata dall'API, dalla più recente.
Passa tra **Tutti**, **Attivi** e **Completati**. Ogni riga mostra lo stato,
l'obiettivo, l'avanzamento, i blob letti, i nuovi rilevamenti, quando è
iniziata e quanto è durata. Fai clic su una riga per aprirla; le icone in fondo
riscansionano o eliminano un'operazione terminata.

Un'operazione si trova in uno di sei stati: **in coda**, **in esecuzione**,
**in pausa**, **completato**, **fallito** o **annullato**.

### Dettaglio della scansione

L'intestazione indica l'obiettivo, il tipo di scansione, quando è stata inviata
e da quanto è in esecuzione. I pulsanti cambiano con lo stato:

- **Metti in pausa** (in esecuzione) — i worker smettono di prendere nuovi
  repository; nulla di quanto già trovato va perso. **Riprendi** continua da
  dove si era fermata.
- **Annulla** (qualsiasi operazione non terminata) — interrompe l'operazione
  definitivamente.
- **Riscansiona** (terminata) — rimette in coda lo stesso obiettivo come nuova
  operazione.
- **Elimina** (terminata) — rimuove l'operazione **e tutti i rilevamenti che ha
  registrato**. L'azione non può essere annullata.

Sotto l'intestazione:

- **Contatori** — repository completati sul totale, blob scansionati e nuovi
  rilevamenti (visti per la prima volta da questa operazione).
- **Avanzamento e tempo rimanente** — una stima compare quando ci sono dati
  sufficienti; fino ad allora i contatori sono la verità.
- **Fasi** — Individuazione dei repository → Scansione dei repository (clone,
  percorso della cronologia, rilevamento) → Classificazione dei provider →
  Finalizzazione delle analisi.
- **Pipeline** — un punto per repository, fermo nel passo segnalato per ultimo
  dal suo worker (clone, leggi storia, attribuisci, verifica, salva) e che
  avanza quando viene segnalato il passo successivo; i repository completati
  finiscono nel contenitore a destra. Arancione indica corrispondenze candidate
  finora, rosso un tentativo fallito, grigio un repository saltato.
- **Attività in tempo reale** — trasmessa dai worker mentre la scansione è in
  corso.
- **Riepilogo** — al termine dell'operazione: quanti rilevamenti in quanti
  repository, quanti sono alti o critici, quanti sono ancora raggiungibili dal
  ramo predefinito e quali repository sono coinvolti.
- **Rilevamenti registrati da questa scansione** — l'elenco stesso, con un
  collegamento per aprirlo in Rilevamenti.

### Attività in tempo reale

Il pannello in tempo reale mostra cosa sta facendo ogni worker in questo
momento. Il suo stato indica **connessione**, **in diretta**, **riconnessione**,
**terminato** o **non disponibile**; se il server non trasmette, i contatori
qui sopra si aggiornano comunque ogni pochi secondi.

- **Scoperta** — per le scansioni Utente e Organizzazione, l'elenco dei
  repository dell'account, pagina per pagina. *In attesa del limite di GitHub*
  è una pausa, non un errore.
- **In corso** — ogni repository e il passo in cui si trova: **clone** (bare:
  solo branch e tag), **leggi storia** (ogni versione unica di file, con il
  conteggio progressivo delle corrispondenze candidate), **attribuisci**
  (risalita di ogni corrispondenza al commit che l'ha introdotta), **verifica**
  (convalida attiva, se abilitata) e **salva**.
- **Appena terminati** — repository completati da poco, con i loro nuovi
  rilevamenti, i blob letti e il tempo impiegato.
- **Registro attività** — il flusso completo degli eventi; **Mostra altri** lo
  espande.

Un repository il cui tentativo fallisce viene ritentato automaticamente e mostra
*tentativo 2*, *tentativo 3*… Un repository che fallisce l'ultimo tentativo
viene **saltato**; il resto dell'operazione prosegue.

### Perché una scansione può non trovare nulla di nuovo

Una credenziale viene registrata una sola volta, dalla prima scansione che l'ha
trovata. Scansionare di nuovo lo stesso repository trova le stesse chiavi ma non
registra *nuovi* rilevamenti — restano attribuiti alla scansione precedente.
Apri **Rilevamenti** filtrati su quel repository per vederli tutti.

---

## Panoramica

La pagina principale, aggiornata ogni dieci secondi (l'indicatore **Dal vivo**
mostra l'intervallo).

- **Postura di sicurezza** — un punteggio su 100. Premi **Come si calcola** per
  vedere ogni penalità (sotto).
- **Conteggi principali** — rilevamenti aperti, critici, attivi su HEAD,
  repository esposti, scansioni attive, pattern non mappati.
- **Aperti per gravità**, **Aperti per provider**, **Valutazione** —
  ripartizioni. Ogni riquadro e barra è un collegamento a un elenco filtrato di
  Rilevamenti.
- **Attività recente** — scansioni terminate o fallite, nuovi rilevamenti,
  segnalazioni inviate.
- **Scansioni recenti** — le operazioni più recenti e il loro avanzamento.

### Layout

**Layout** passa tra quattro disposizioni: **Tutto**, **Triage quotidiano**
(conteggi e ripartizioni, senza punteggio), **Sintesi esecutiva** (punteggio,
numeri chiave, cosa è cambiato) e **Operazioni di sicurezza** (prima attività e
scansioni in corso). **Personalizza** ti permette di spostare un pannello su o
giù con le frecce, nasconderlo con l'occhio e ripristinarne uno nascosto
dall'elenco sottostante. I layout vengono salvati in questo browser.

### Come si calcola il punteggio di postura

Il punteggio parte da 100 e perde punti per ciascun fattore. Ogni penalità si
satura, quindi i primi rilevamenti costano di più e nessun singolo fattore può
prendere il sopravvento:

- **Rilevamenti critici** — fino a 30 punti.
- **Raggiungibili da HEAD** — fino a 25 punti. Il segnale più netto: chiunque
  cloni oggi il ramo predefinito ottiene la credenziale.
- **Rilevamenti alti** — fino a 15 punti.
- **Repository esposti** — fino a 15 punti, in base alla quota di repository
  scansionati che hanno un rilevamento aperto.
- **Quota irrisolta** — fino a 10 punti, in base alla quota di rilevamenti
  ancora aperti. Un rilevamento chiuso non costa nulla.
- **Proprietari non avvisati** — fino a 5 punti, per i repository esposti a cui
  non è stata inviata alcuna segnalazione.

Da 85 in su si legge **Sotto controllo**, 65–84 **Da migliorare**, 40–64
**Esposta**, sotto 40 **Critica**. Se nulla è stato scansionato non c'è alcun
punteggio, invece di un 100 fuorviante. Il punteggio è calcolato nel browser dai
contatori dell'intero database del server, mai da una singola pagina di
rilevamenti.

---

## Rilevamenti

L'elenco completo di ogni credenziale registrata, 50 righe per pagina.

### Filtri

- **Cerca** — percorso, regola, provider o repository.
- **Gravità**, **Provider**, **Stato** (Aperto, Ruotato, Falso positivo,
  Ignorato).
- **Esposizione** — su HEAD, solo nella cronologia o confermata attiva.
- **Ordine** — predefinito, rilevati più di recente, commit più vecchio o
  gravità.
- **Un repository** / scansione — compare come chip quando arrivi da un
  repository o da una scansione; rimuovilo per vedere tutto.

Il filtro è memorizzato nell'URL, quindi una vista filtrata può essere salvata
tra i preferiti, condivisa o riaperta dopo un ricaricamento. **Cancella filtri**
lo azzera.

Esposizione e Ordine sono calcolati nel browser. Con uno dei due attivo, la
pagina carica fino a 500 rilevamenti dal server e li affina; se ce n'erano di
più, lo indica sopra la tabella invece di mostrare un totale fuorviante.

### Filtri predefiniti e salvati

Il menu **Filtri** offre viste pronte:

- **Critici, ancora aperti** — da ruotare per primi.
- **Raggiungibili da HEAD** — leggibili da chiunque cloni oggi.
- **Confermate attive** — il probe ha ricevuto una risposta valida.
- **Non attribuiti** — nessuna regola di provider li ha rivendicati.
- **Scoperti di recente** — prima i rilevamenti più recenti.
- **Irrisolti da più tempo** — prima il commit più vecchio.

**Salva il filtro corrente** memorizza con un nome il filtro impostato; può
essere rinominato o eliminato dallo stesso menu ed è proposto anche nella
palette dei comandi. I filtri salvati restano solo in questo browser e
contengono solo i campi del filtro — mai un testo di ricerca che sembri un
segreto incollato.

### Leggere una riga

- **Gravità** e il **provider** a cui appartiene la credenziale (oppure
  *non mappato*).
- **Credenziale** — un'anteprima mascherata. Il valore reale non lascia mai il
  motore, a meno che tu non abbia attivato esplicitamente la visualizzazione in
  chiaro (vedi Impostazioni).
- **Repository** e **Posizione** — il file e la riga, con un collegamento al
  file su GitHub.
- **Esposizione** — `su HEAD` significa che è ancora presente nell'albero
  corrente; `cronologia` significa che è stata rimossa ma è ancora recuperabile
  clonando. *Commit sconosciuto — riscansiona per attribuire* significa che il
  commit che l'ha introdotta non è stato individuato; una nuova scansione lo
  completa.
- **ATTIVO — ruota** — la convalida attiva ha confermato che la chiave funziona
  ancora.
- **Stato** e ora di **Rilevato**.

Il menu alla fine di una riga contrassegna un rilevamento come **Aperto**,
**Ruotato**, **Falso positivo** o **Ignorato**.

### Dettagli del rilevamento

Fai clic su una riga per aprire il pannello di dettaglio. Il suo indirizzo
finisce nell'URL, quindi **Copia il link** dà a un collega la stessa vista;
**Copia il riferimento** copia un breve riferimento per un ticket. Il pannello
mostra:

- **Credenziale** — anteprima mascherata, categoria e validità. Quando il valore
  è stato conservato per un proprietario non mascherato e il mascheramento è
  disattivato, **Mostra** lo rivela, contrassegnato **IN CHIARO**.
- **Quota e limiti** — ciò che il provider ha riportato durante il controllo di
  validità: richieste o token rimasti, ambiti, paese, valuta. Un'istantanea al
  momento della scansione.
- **Perché è stato rilevato** — la regola, il provider a cui è stato attribuito,
  la lunghezza e il set di caratteri, una variabile o un nome host vicini e
  l'esposizione. La spiegazione usa solo forma, lunghezza e contesto, mai il
  valore.
- **Dove** — file, commit, autore e data del commit, con un collegamento alla
  riga su GitHub.
- **Cronologia** — commit, rilevamento, verifica e decisioni di valutazione.
- **Cosa fare** — i passi di rimedio in ordine: revocare e riemettere presso il
  provider, controllarne il log di audit, rimuovere il valore da HEAD, decidere
  se riscrivere la cronologia e aggiungere un controllo dei segreti pre-commit.
- **Stesso repository** e **Stesso provider** — rilevamenti correlati, a un
  clic di distanza.
- **Metadati** — tutto il resto registrato dal motore.

### Valutazione in blocco

Seleziona le caselle all'inizio delle righe, oppure la casella dell'intestazione
per selezionare ogni riga della pagina. In basso compare una barra con il
conteggio e i quattro stati. **Ruotato** e **Falso positivo** chiedono prima una
conferma, perché ciascuno è un'affermazione sul mondo esterno — che la chiave è
stata revocata, che la corrispondenza era sbagliata — e in blocco è
l'affermazione che può chiudere silenziosamente una fuga reale. Aperto e
Ignorato si applicano subito. **Esporta selezionati** esporta solo le righe
selezionate.

### Esportazione

**Esporta** scrive i rilevamenti attualmente caricati, registrando nel file i
filtri che li hanno prodotti:

- **Rilevamenti in CSV** — una riga per rilevamento, per un foglio di calcolo.
- **Rilevamenti in JSON** — strutturato, per uno script.
- **Rapporto d'incidente** — Markdown, raggruppato per repository.
- **Sintesi esecutiva** — una pagina, senza tabelle.

Le esportazioni contengono solo anteprime mascherate; la credenziale in chiaro
non viene mai scritta su file, in nessun formato.

---

## Repository

Ogni repository scansionato, classificato in base alla gravità della sua
esposizione aperta.

- **Filtra per proprietario/nome** e **Solo con rilevamenti aperti**.
- **Ordina per** rischio, rilevamenti aperti, ultima scansione o nome.
- Colonne: tipo, conteggi aperti per gravità, aperti / totali, **Rischio**,
  **Prossimo passo**, **Segnalato** e ultima scansione.
- Fai clic su un repository per aprirne i rilevamenti.

Il **Rischio** va da 0 (nulla di aperto) a 100. I rilevamenti aperti sono pesati
per gravità — un critico conta dieci volte un basso — e il totale si satura,
quindi un repository con un solo critico è già in alto nella classifica. Fasce:
Basso, Medio (25+), Alto (50+), Critico (75+). Passa il mouse per vedere i
conteggi che lo determinano.

Il **Prossimo passo** indica l'unica cosa che vale la pena fare: *Ruota le
chiavi critiche*, *Avvisa il proprietario*, *Segnalazione in coda*,
*Segnalazione fallita — riprova*, *Rivedi i rilevamenti restanti* o *Niente di
aperto*.

### Avvisare il proprietario

Il chip **Email di segnalazione** in alto mostra se le email sono attive e
rimanda alle Impostazioni. Con le email attive, **Segnala** su una riga mette in
coda una segnalazione al proprietario del repository, con l'elenco dei suoi
rilevamenti in anteprima mascherata. La colonna **Segnalato** indica poi *in
invio*, *inviata*, *fallita* o *esclusa*.

### Macchina del tempo

L'icona dell'orologio su una riga (o il chip **Macchina del tempo** in
Rilevamenti filtrati su un solo repository) riproduce l'esposizione di quel
repository commit per commit. Ogni file con una credenziale ha una corsia; un
rombo segna il commit che ha introdotto ciascuna chiave, e da lì una linea arriva
a oggi — continua finché la chiave è ancora su HEAD, sfumata una volta rimossa.

Trascina il cursore, avanza con i pulsanti precedente e successivo, oppure premi
play per scorrere in avanti dalla prima fuga. A ogni passo la pagina dice quante
credenziali erano state committate fino a quel momento, quante sono ancora su
HEAD oggi e che tutte restano clonabili. Sotto, i commit che hanno introdotto una
credenziale sono elencati dal più recente, con autore, data e un collegamento a
ciascun rilevamento.

La scansione sa quando una chiave è stata committata, non quando è stata
cancellata, quindi la linea di una chiave rimossa sfuma invece di finire a una
data inventata. I rilevamenti ancora senza data di commit sono contati sotto
l'elenco; una nuova scansione li colloca.

---

## Diffusione chiavi

Credenziali trovate in più di un repository. I rilevamenti sono registrati una
volta per repository, quindi lo stesso segreto in più posti compare solo qui,
dove le copie sono collegate dalla loro impronta.

- **Chiavi condivise**, **Tra proprietari** (chiavi in repository di proprietari
  diversi), **Aperte su HEAD** e **Repository / proprietari** toccati. Quando ci
  sono più chiavi condivise di quante il grafo ne disegni, la prima scheda dice
  quante sono disegnate.
- **Trovata in** — mostra le chiavi presenti in 2, 3, 5 o 10 repository o più.
- **Il grafo** — un rombo per chiave, colorato per gravità e dimensionato in base
  a quanto si è diffusa; un cerchio per repository; un quadrato per ogni
  proprietario che ne ha due o più. Una linea rossa è una copia ancora su HEAD,
  una tratteggiata una copia già gestita. Passa il mouse per illuminare i vicini
  di una chiave.
- **L'elenco** accanto contiene le stesse chiavi, per l'uso da tastiera e con
  lettore di schermo. Scegliere una chiave in uno dei due mostra i suoi
  repository, ciascuno collegato al proprio rilevamento.

Una sola rotazione sistema ogni copia, quindi il dettaglio di una chiave può
**segnare ogni copia** come *Ruotato* o *Falso positivo* in un colpo solo, dopo
una conferma. Un valore incollato nei repository di molti proprietari non
collegati è di solito codice boilerplate che il passaggio generico ha scambiato
per un segreto — nomi di classi di un framework, chiavi di esempio di un
tutorial — e questa vista è il modo più rapido per individuarli.

---

## Non mappati

Il backlog di segreti ad alta entropia che nessuna regola di provider specifica
rivendica, raggruppati per forma. Ogni riga mostra un **nome presunto** (preso
dalla variabile o chiave più vicina — a volte solo una parola nei dintorni), un
**campione** mascherato, la **forma** (lunghezza e set di caratteri), il
**contesto** in cui è stato trovato, quante volte è stato **rilevato**, il suo
stato e quando è stato visto l'ultima volta. Una forma che continua a comparire
è un provider per cui vale la pena scrivere una regola. I rilevamenti di questo
tipo appaiono come *non mappato* finché una regola non li rivendica; quando
succede, la scansione successiva li riattribuisce.

---

## Provider

Ogni provider che GitScanner può rilevare e attribuire, raggruppato per
categoria (AI, cloud, dati, pagamenti, messaggistica, DevOps, strumenti per
sviluppatori, altro), con il numero di regole per ciascuno. Fai clic su un
provider per vedere:

- **Perché questa gravità** — critico è controllo totale; alto è accesso ampio
  alle API; medio è accesso ristretto o limitato a un host; basso identifica un
  account ma non può agire su di esso.
- **Cosa permette di fare una fuga** — l'impatto per quella categoria.
- **Come sono fatte le chiavi e come vengono rilevate**, una scheda per regola:
  - *prefisso proprio* — il valore inizia con il prefisso del provider, che è
    di per sé una prova;
  - *nome impostazione* — un valore senza una forma propria, segnalato solo
    quando è assegnato a un'impostazione con il nome del provider;
  - *URL di connessione* — un URL con la credenziale al suo interno; uno che
    punta solo a localhost o a un servizio compose è trattato come
    configurazione di sviluppo e scartato;
  - *blocco PEM* — una chiave privata da BEGIN a END;
  - *solo struttura* — lunghezze fisse, separatori o marcatori, mantenuta
    volutamente ristretta.
  Ogni scheda può mostrare la struttura della chiave, il pattern, il testo che il
  file deve contenere e una parola che deve comparire nelle vicinanze.
- **Dopo una corrispondenza** — segnaposto (`your-…-key`, `xxxx`, `changeme`),
  chiavi di documentazione e sandbox e fixture di test vengono scartati; ciò che
  sopravvive è conservato solo come impronta e anteprima mascherata.

Un provider mostra la sua regola più grave. Su ogni rilevamento la gravità sale
di un livello se la fuga è ancora raggiungibile da HEAD, e di un altro per i
provider cloud e di pagamento, fino a critico.

---

## Analitica

Una lettura di tutto ciò che è stato scansionato finora:

- **Rilevamenti** e **Repository** — totali, aperti, esposti / scansionati.
- **Calendario delle fughe** — un quadrato per giorno, un anno alla volta,
  ombreggiato in base a quante credenziali sono state committate quel giorno e
  bordato dove una era critica. Scegli l'anno in alto a destra. I giorni sono in
  UTC: il fuso orario dell'autore non è registrato, quindi un quadrato può
  corrispondere al suo giorno precedente o successivo.
- **Cronologia introduzione leak** — rilevamenti in base al mese in cui la
  credenziale è stata committata.
- **Aperti per gravità**, **Aperti per provider** (provider noti più la
  categoria per i non mappati), **Per tipo di applicazione**, **Stato di
  valutazione**.
- **Regole di rilevamento** — quali regole scattano più spesso.
- **Convalida credenziali attive** — attive, revocate, nessun prober, verifica
  fallita.
- **Email di segnalazione** — repository contattati ed esposti non ancora
  segnalati.
- **Repository più esposti** — secondo lo stesso punteggio di rischio di
  Repository.
- **Irrisolti da più tempo** — rilevamenti aperti per data di commit.

I dati sono complessivi; non c'è un intervallo di date, perché l'API non lo
offre. I filtri in alto guidano i collegamenti a Rilevamenti aperti dalla
pagina.

---

## Email di segnalazione (Notifiche)

Ogni segnalazione inviata ai proprietari dei repository e il suo esito:
repository, destinatario, stato (*in invio*, *inviata*, *fallita*, *esclusa*),
numero di rilevamenti, quando è stata richiesta e quando è stata inviata. Cerca
per repository o destinatario.

- **Vedi email** — il messaggio in HTML e in testo semplice, rigenerato dai
  rilevamenti così come sono ora.
- **Reinvia** — lo rimette in coda.

---

## Impostazioni

### Aspetto

Salvato solo in questo browser.

- **Tema** — Carta (carta invecchiata, verde acqua e rosso sangue di bue),
  Mezzanotte (terminale quasi nero, verde fosforo), Oceano (blu navy, ciano e
  blu), Segnale (grafite, solo gli avvisi sono colorati), Minimale (stile
  aziendale essenziale).
- **Chiaro o scuro** — Chiaro, Scuro, oppure Sistema per seguire il sistema
  operativo.
- **Densità** — Compatta mostra più righe sullo schermo.
- **Animazioni** — Ridotte disattiva le animazioni.
- **Avvia con la barra laterale ridotta**.
- **Reimposta l'aspetto** — torna ai valori predefiniti.

### Introduzione

**Rivedi l'introduzione** riapre il tour introduttivo.

### Notifiche del browser

Con questa opzione attiva e il permesso del browser concesso, una dashboard
aperta — basta una scheda in background — ti avvisa quando una scansione che
stava seguendo finisce e quando altre chiavi risultano attive; un clic sulla
notifica apre la scansione o le chiavi attive. Mentre stai guardando la
dashboard, la stessa notizia arriva invece come messaggio nella pagina. Nulla
viene inviato quando tutte le schede della dashboard sono chiuse. Salvato solo in
questo browser.

### Installare la dashboard

La dashboard può essere installata come app dalla barra degli indirizzi del
browser (o con *Aggiungi a schermata Home* su un telefono). Si apre allora in una
finestra propria e la sua struttura si carica senza rete; i dati arrivano
comunque dall'API e non vengono mai conservati nella cache del browser.

### Email di segnalazione

L'interruttore generale. Quando è attivo, Segnala invia una segnalazione al
proprietario del repository. Quando è disattivo non viene inviato nulla e tutto
ciò che era già in coda viene scartato. Non esistono ancora preferenze per
gravità o di riepilogo.

### Mascheramento di sicurezza e proprietari non mascherati

**Attivo** (predefinito): ogni credenziale è mostrata mascherata.
**Disattivo**: le credenziali trovate nei repository dei proprietari elencati in
**Proprietari non mascherati** sono mostrate per intero, a scopo di test; per
tutti gli altri restano mascherate, e le email e le esportazioni sono sempre
mascherate.

- Aggiungi solo utenti o organizzazioni GitHub di tua proprietà.
- I valori vengono conservati, cifrati, a partire dalla scansione successiva —
  riscansiona i repository di un proprietario per vedere in chiaro i
  rilevamenti esistenti.
- Rimuovere un proprietario elimina tutto ciò che era conservato per lui.
- Se il server non ha `SECRET_SEAL_KEY`, la visualizzazione in chiaro non è
  disponibile e non viene mai conservato nulla. Generane una con
  `openssl rand -hex 32`, aggiungila a `.env.local` e riavvia.

Finché il mascheramento è disattivato, sulla pagina resta un avviso rosso: le
credenziali complete dei proprietari elencati sono visibili a chiunque possa
aprire questa dashboard.

---

## Convalida attiva

Quando l'operatore la abilita (`PROBE_SECRETS`) per i propri obiettivi,
GitScanner effettua per ogni rilevamento una chiamata di identità in sola
lettura per verificare se la chiave funziona ancora e a quale account
appartiene:

- **Attivo — ruota** — il provider l'ha accettata. Ruota questa per prima.
- **Revocato** — il provider l'ha rifiutata; già ruotata o revocata.
- **Nessun prober** — non esiste un controllo per questo provider.
- **Verifica fallita** — limite di frequenza o rete; riprova con una nuova
  scansione più tardi.

È disattivata per impostazione predefinita, non enumera mai i contenuti
dell'account e le sue letture di quota e limiti compaiono nel pannello del
rilevamento.

---

## Gravità

- **Critico** — controllo totale: spostare denaro, prendere il controllo
  dell'account, leggere tutto ciò che protegge.
- **Alto** — ampio accesso alle API come l'account.
- **Medio** — accesso ristretto o limitato a un host: un canale, un webhook, un
  server che deve anche essere raggiungibile.
- **Basso** — identifica un account ma non può agire su di esso, come una
  chiave in modalità test. Vale comunque la pena ruotarla.

---

## Risoluzione dei problemi

- **“Impossibile raggiungere l'API”** — controlla l'indirizzo nella barra
  superiore e che lo stack sia in esecuzione; il punto diventa verde quando
  risponde.
- **Una scansione resta allo 0%** — aprila e guarda l'Attività in tempo reale.
  *In attesa del limite di GitHub* si risolve da solo; nessuna attività
  significa che i worker non sono in esecuzione.
- **Una nuova scansione non ha trovato nulla di nuovo** — i rilevamenti
  appartengono alla scansione precedente; filtra invece Rilevamenti per
  repository.
- **Commit sconosciuto** — riscansiona il repository per attribuirlo.
- **Segnala è disattivato** — attiva le email di segnalazione nelle
  Impostazioni.
- **Mostra manca nel pannello** — il mascheramento è attivo, il proprietario
  non è nell'elenco dei non mascherati, il server non ha `SECRET_SEAL_KEY`,
  oppure il repository non è stato riscansionato dopo l'aggiunta del
  proprietario.
- **L'elenco dice di aver affinato solo 500 rilevamenti** — restringi i filtri
  lato server (gravità, provider, stato, repository) in modo che la finestra
  copra tutto.
- **Nessun “Chiedi a Claude” nella palette** — il server non ha
  `ANTHROPIC_API_KEY`, oppure la ricerca è di una sola parola (quella è una
  ricerca semplice).
- **Chiedi a Claude ha rifiutato la domanda** — conteneva qualcosa con la forma
  di una credenziale; riformulala senza il valore.
- **Nessuna notifica del browser** — controlla il permesso per questo sito nel
  browser; a un sito bloccato non si può chiedere di nuovo dalla pagina.
- **Risultati obsoleti** — eliminare un'operazione o cancellare i dati ne
  rimuove i rilevamenti; riscansiona per recuperarli.

GitScanner è destinato esclusivamente a test di sicurezza autorizzati. Scansiona
ciò che possiedi o che hai il permesso di testare, e informa i proprietari di
ciò che trovi.
