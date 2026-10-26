# Utilizzo di GitScanner

GitScanner clona l'intera cronologia di un repository, analizza ogni singolo blob e
segnala le credenziali individuate — sia nell'albero corrente che in qualsiasi punto del passato.
Questa pagina è una guida rapida alla dashboard.

## Connessione

L'indirizzo dell'API si trova nella barra superiore; il punto accanto ad esso è verde quando la
dashboard riesce a raggiungerlo. Indirizzalo al tuo host API se non stai utilizzando il valore
predefinito `http://localhost:8080`. L'indirizzo, il tema e la lingua vengono
memorizzati in questo browser.

Usa i controlli in alto a destra per passare dalla **modalità chiara a quella scura** e
per scegliere la tua **lingua**.

## Avvio di una scansione

Premi **Nuova scansione** (in alto a sinistra o nella barra laterale). Scegli cosa scansionare:

- **Repository** — un URL di clone HTTPS, ad es. `https://github.com/owner/repo`.
- **Utente** o **Organizzazione** — ogni repository posseduto da tale account. Seleziona *includi repository derivati (fork)*
  per scansionare anche i repository forkati.

La scansione viene eseguita in background. Si aprirà la vista dettagliata della scansione, dove
la barra di avanzamento e i contatori si aggiornano man mano che i repository vengono analizzati.

## Lettura dei risultati

La **Panoramica** è la schermata principale: rilevamenti aperti, quanti sono critici, quanti sono
ancora attivi su HEAD e le suddivisioni per gravità, provider e stato di valutazione.
Ogni riquadro e barra è un collegamento a un elenco filtrato di Rilevamenti.

L'**Analitica** approfondisce i dati — una cronologia dell'introduzione delle credenziali e
suddivisioni per tipo di applicazione e regola di rilevamento.

I **Rilevamenti** costituiscono l'elenco completo e filtrabile. Filtra per gravità, provider, stato
o testo libero; il filtro è memorizzato nell'URL, consentendo di condividere o salvare tra i preferiti
una vista filtrata. Ciascuna riga mostra:

- la gravità e il provider a cui appartiene la credenziale;
- un'anteprima mascherata — il vero segreto non lascia mai il motore;
- il file e la riga, con un collegamento al file su GitHub;
- il commit che l'ha introdotta, il relativo autore e la data;
- **Esposizione** — `su HEAD` significa che è ancora presente nell'albero attuale; `cronologia`
  significa che è stata rimossa ma è ancora recuperabile clonando;
- un badge **Attivo — ruota** quando la convalida attiva ha confermato che la chiave funziona ancora.

Usa il menu alla fine di ciascuna riga per contrassegnare un rilevamento come **Ruotato**, **Falso
positivo** o **Ignorato** man mano che procedi con la verifica.

## Repository e pattern non mappati

I **Repository** classificano ogni repository scansionato in base alla sua esposizione aperta,
suddivisa per gravità. I **Non mappati** rappresentano il backlog di segreti ad alta entropia che
nessuna regola specifica ha ancora reclamato, raggruppati per pattern — un pattern frequente è spesso
un provider per cui vale la pena scrivere una regola.

## Convalida attiva

Se l'amministratore l'ha abilitata per i propri target, GitScanner effettua una
chiamata di sola lettura sull'identità per ciascun rilevamento, al fine di verificare se la chiave
funziona ancora e a quale account appartiene. Una chiave confermata attiva è quella da ruotare per prima.
È disattivata per impostazione predefinita e non enumera mai i contenuti dell'account.
