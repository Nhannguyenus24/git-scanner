# Utilisation de GitScanner

GitScanner clone l'historique complet d'un dépôt, lit une seule fois chaque
version unique de fichier (blob) et signale les identifiants qu'il trouve —
dans l'arbre actuel comme n'importe où dans le passé. Une clé supprimée il y a
trois ans reste lisible par quiconque clone le dépôt ; c'est donc toujours une
fuite tant qu'elle n'est pas révoquée.

Ce guide parcourt le tableau de bord page par page. En bref :

1. Vérifiez que la pastille à côté de l'adresse de l'API, dans la barre supérieure, est verte.
2. Cliquez sur **Nouveau scan** et indiquez un dépôt, un utilisateur ou une organisation.
3. Suivez la page du scan jusqu'à ce qu'il se termine.
4. Traitez les **Résultats** : renouvelez ce qui est actif, marquez ce qui n'est pas une fuite.
5. Prévenez les propriétaires par un e-mail de signalement depuis **Dépôts**.

---

## L'écran

### Barre supérieure

- **Adresse de l'API** — le serveur d'où provient tout ce qui s'affiche. La
  pastille à côté est grise pendant la vérification, verte quand l'API répond
  et rouge quand elle est injoignable. Saisissez votre propre hôte si vous
  n'utilisez pas l'adresse par défaut `http://localhost:8080` ; elle est
  conservée dans ce navigateur.
- **Commandes** — ouvre la palette de commandes (voir ci-dessous).
- **Aide** — cette page.
- **Clair / sombre** — bascule entre papier clair et papier sombre.
- **Langue** — English, Tiếng Việt, Italiano, 中文, Español, Deutsch et
  Français. Tout, y compris ce guide, change avec elle.

### Barre latérale

Chaque page est à un clic. Trois entrées affichent un compteur en direct :
**Résultats** (résultats ouverts), **Scans** (scans encore en cours) et
**Non mappés** (formes en attente d'une règle de fournisseur). Repliez la barre
latérale pour ne garder que les icônes avec la flèche en bas ou avec `Ctrl+[`
(`⌘[` sur Mac). Sur un téléphone, la barre latérale devient une barre en bas
de l'écran ; les pages moins utilisées se trouvent sous **Plus**.

### Clavier

- `Ctrl+K` / `⌘K` — ouvre ou ferme la palette de commandes, depuis n'importe où.
- `/` — ouvre la palette de commandes.
- `N` — lance un nouveau scan.
- `Ctrl+[` / `⌘[` — replie ou déplie la barre latérale.
- `Esc` — ferme une boîte de dialogue, un panneau ou la palette.

`N` et `/` sont ignorés pendant la saisie dans un champ de texte, ils ne
volent donc jamais une frappe destinée à un filtre.

### Palette de commandes

Un seul champ pour tout ce que le tableau de bord sait faire au clavier. Tapez
pour réduire la liste, déplacez-vous avec `↑` `↓` (ou `Ctrl+N` / `Ctrl+P`),
exécutez avec `Enter`. Elle propose :

- **Actions** — lancer un nouveau scan ; exporter la vue actuelle.
- **Aller à** — toutes les pages.
- **Filtres** — les préréglages de résultats et les filtres que vous avez
  enregistrés, appliqués en une seule étape.
- **Apparence** — clair ou sombre, suivre le système, chaque thème, densité,
  animations, barre latérale et réinitialisation.
- **Langue** — change la langue de l'interface.

Tout autre texte saisi devient **Rechercher « … » dans les détections**, qui
ouvre Résultats avec ce texte comme filtre de recherche. La palette ne garde
volontairement aucun historique : un champ de recherche qui se souvient est un
endroit où un identifiant collé pourrait rester.

### Introduction

À la première ouverture, le tableau de bord propose une introduction en sept
étapes. Passez-la quand vous voulez ; revoyez-la plus tard depuis
**Paramètres → Introduction**.

---

## Démarrer un scan

Cliquez sur **Nouveau scan** (barre latérale, page Vue d'ensemble ou Scans, ou
`N`). Choisissez une cible :

- **Dépôt** — une URL de clonage HTTPS, par exemple
  `https://github.com/owner/repo`. Les URL SSH et celles qui contiennent un nom
  d'utilisateur ou un jeton sont refusées.
- **Utilisateur** — tous les dépôts d'un utilisateur GitHub. Un identifiant se
  compose de lettres, de chiffres et de tirets, de 1 à 39 caractères.
- **Organisation** — tous les dépôts d'une organisation GitHub.
- **Aléatoire** — un échantillon aléatoire de dépôts publics, réservé aux tests
  de détection. Indiquez combien, ou cochez **Exécuter en continu** pour
  continuer l'échantillonnage jusqu'à ce que vous le mettiez en pause ou
  l'annuliez.

Pour Utilisateur, Organisation et Aléatoire, cochez **Inclure les dépôts forké
(forks)** pour scanner aussi les forks ; ils sont ignorés par défaut, car un
fork répète surtout son dépôt d'origine.

Cliquez sur **Démarrer le scan** ou appuyez sur `Enter`. Le scan est mis en
file d'attente en arrière-plan et le tableau de bord ouvre sa page de détail.

---

## Suivre un scan

### Liste des scans

**Scans** liste chaque tâche acceptée par l'API, de la plus récente à la plus
ancienne. Basculez entre **Tous**, **Actifs** et **Terminés**. Chaque ligne
affiche l'état, la cible, la progression, les blobs lus, les nouveaux
résultats, l'heure de démarrage et la durée. Cliquez sur une ligne pour
l'ouvrir ; les icônes en fin de ligne relancent ou suppriment une tâche
terminée.

Une tâche se trouve dans l'un de six états : **en attente**, **en cours**,
**en pause**, **terminé**, **échoué** ou **annulé**.

### Détail du scan

L'en-tête indique la cible, le type de scan, l'heure de soumission et la durée
d'exécution. Les boutons changent selon l'état :

- **Mettre en pause** (en cours) — les workers cessent de prendre de nouveaux
  dépôts ; rien de ce qui a déjà été trouvé n'est perdu. **Reprendre** repart
  de là où il s'était arrêté.
- **Annuler** (toute tâche non terminée) — arrête la tâche définitivement.
- **Rescanner** (terminée) — remet la même cible en file sous forme de nouvelle
  tâche.
- **Supprimer** (terminée) — supprime la tâche **et tous les résultats qu'elle
  a enregistrés**. Cette action est irréversible.

Sous l'en-tête :

- **Compteurs** — dépôts traités sur le total, blobs scannés et nouveaux
  résultats (vus pour la première fois par cette tâche).
- **Progression et temps restant** — une estimation apparaît dès qu'il y a
  assez de données ; d'ici là, les compteurs font foi.
- **Étapes** — Découverte des dépôts → Scan des dépôts (clonage, parcours de
  l'historique, détection) → Classement des fournisseurs → Finalisation des
  analyses.
- **Activité en direct** — transmise par les workers pendant le scan.
- **Bilan** — à la fin de la tâche : combien de résultats dans combien de
  dépôts, combien sont élevés ou critiques, combien restent atteignables depuis
  la branche par défaut et quels dépôts sont concernés.
- **Résultats enregistrés par ce scan** — la liste elle-même, avec un lien pour
  l'ouvrir dans Résultats.

### Activité en direct

Le panneau en direct montre ce que fait chaque worker en ce moment. Son statut
indique **connexion**, **en direct**, **reconnexion**, **terminé** ou
**indisponible** ; si le serveur ne diffuse pas, les compteurs ci-dessus se
rafraîchissent malgré tout toutes les quelques secondes.

- **Découverte** — pour les scans Utilisateur et Organisation, la liste des
  dépôts du compte, page par page. *En attente de la limite GitHub* est une
  pause, pas un échec.
- **En cours** — chaque dépôt et l'étape où il se trouve : **clone** (bare :
  branches et tags seulement), **lire l'historique** (chaque version unique de
  fichier, avec le décompte courant des correspondances candidates),
  **attribuer** (rattacher chaque correspondance au commit qui l'a introduite),
  **vérifier** (validation en direct, si elle est activée) et **enregistrer**.
- **Juste terminés** — les dépôts terminés il y a un instant, avec leurs
  nouveaux résultats, les blobs lus et le temps passé.
- **Journal d'activité** — le flux complet des événements ; **Afficher … de
  plus** le déplie.

Un dépôt dont la tentative échoue est relancé automatiquement et affiche
*tentative 2*, *tentative 3*… Un dépôt qui échoue à sa dernière tentative est
**ignoré** ; le reste de la tâche continue.

### Pourquoi un scan peut ne rien trouver de nouveau

Un identifiant n'est enregistré qu'une fois, par le premier scan qui l'a
trouvé. Rescanner le même dépôt retrouve les mêmes clés mais n'enregistre aucun
*nouveau* résultat — ils restent attribués au scan précédent. Ouvrez
**Résultats** filtrés sur ce dépôt pour les voir tous.

---

## Vue d'ensemble

La page principale, actualisée toutes les dix secondes (le repère **En direct**
indique l'intervalle).

- **Posture de sécurité** — un score sur 100. Cliquez sur **Comment c'est
  calculé** pour voir chaque pénalité (ci-dessous).
- **Chiffres clés** — résultats ouverts, critiques, actifs sur HEAD, dépôts
  exposés, scans actifs, formes non mappées.
- **Ouverts par gravité**, **Ouverts par fournisseur**, **Triage** —
  répartitions. Chaque tuile et chaque barre est un lien vers une liste filtrée
  de Résultats.
- **Activité récente** — scans terminés ou échoués, nouveaux résultats,
  signalements envoyés.
- **Scans récents** — les dernières tâches et leur progression.

### Dispositions

**Disposition** bascule entre quatre agencements : **Tout**, **Tri quotidien**
(comptes et répartitions, sans score), **Synthèse de direction** (score,
chiffres clés, ce qui a changé) et **Opérations de sécurité** (d'abord
l'activité et les scans en cours). **Personnaliser** permet de monter ou
descendre un panneau avec les flèches, de le masquer avec l'œil et de faire
revenir un panneau masqué depuis la liste en dessous. Les dispositions sont
enregistrées dans ce navigateur.

### Comment le score de posture est calculé

Le score part de 100 et perd des points pour chaque facteur. Chaque pénalité
sature, de sorte que les premiers résultats coûtent le plus et qu'aucun facteur
ne peut s'emballer à lui seul :

- **Détections critiques** — jusqu'à 30 points.
- **Atteignables depuis HEAD** — jusqu'à 25 points. Le signal le plus net :
  quiconque clone aujourd'hui la branche par défaut obtient l'identifiant.
- **Détections élevées** — jusqu'à 15 points.
- **Dépôts exposés** — jusqu'à 15 points, selon la part des dépôts scannés qui
  ont un résultat ouvert.
- **Part non résolue** — jusqu'à 10 points, selon la part des résultats encore
  ouverts. Un résultat fermé ne coûte rien.
- **Propriétaires non prévenus** — jusqu'à 5 points, pour les dépôts exposés
  sans aucun signalement envoyé.

À partir de 85, le score indique **Maîtrisée**, 65–84 **À améliorer**, 40–64
**Exposée**, en dessous de 40 **Critique**. Sans aucun scan, il n'y a pas de
score du tout plutôt qu'un 100 trompeur. Le score est calculé dans le
navigateur à partir des compteurs de toute la base du serveur, jamais à partir
d'une seule page de résultats.

---

## Résultats

La liste complète de chaque identifiant enregistré, 50 lignes par page.

### Filtrer

- **Rechercher** — chemin, règle, fournisseur ou dépôt.
- **Gravité**, **Fournisseur**, **Statut** (Ouvert, Renouvelé, Faux positif,
  Ignoré).
- **Exposition** — sur HEAD, historique seulement ou confirmé actif.
- **Ordre** — par défaut, détectés en dernier, commit le plus ancien ou
  gravité.
- **Un seul dépôt** / scan — apparaît comme une puce quand vous arrivez depuis
  un dépôt ou un scan ; retirez-la pour tout voir.

Le filtre est stocké dans l'URL : une vue filtrée peut donc être mise en
favori, partagée ou rouverte après un rechargement. **Effacer les filtres** le
réinitialise.

Exposition et Ordre sont calculés dans le navigateur. Si l'un des deux est
actif, la page charge jusqu'à 500 résultats depuis le serveur et les affine ;
s'il y en avait davantage, elle l'indique au-dessus du tableau plutôt que
d'afficher un total trompeur.

### Préréglages et filtres enregistrés

Le menu **Filtres** propose des vues toutes prêtes :

- **Critiques, encore ouverts** — à renouveler en premier.
- **Atteignables depuis HEAD** — lisibles par quiconque clone aujourd'hui.
- **Confirmées actives** — la sonde a reçu une réponse valide.
- **Non attribués** — aucune règle de fournisseur ne les revendique.
- **Découverts récemment** — les détections les plus récentes d'abord.
- **Non résolus depuis le plus longtemps** — le commit le plus ancien d'abord.

**Enregistrer le filtre actuel** enregistre sous un nom le filtre que vous
avez défini ; il peut être renommé ou supprimé depuis le même menu et est aussi
proposé dans la palette de commandes. Les filtres enregistrés restent dans ce
navigateur uniquement et ne contiennent que des champs de filtre — jamais un
texte de recherche qui ressemble à un secret collé.

### Lire une ligne

- **Gravité** et le **fournisseur** auquel appartient l'identifiant (ou
  *non mappé*).
- **Identifiant** — un aperçu masqué. La vraie valeur ne quitte jamais le
  moteur, sauf si vous avez explicitement activé l'affichage en clair (voir
  Paramètres).
- **Dépôt** et **Emplacement** — le fichier et la ligne, avec un lien vers le
  fichier sur GitHub.
- **Exposition** — `sur HEAD` signifie qu'il est toujours dans l'arbre actuel ;
  `historique` signifie qu'il a été supprimé mais reste accessible en clonant.
  *Commit inconnu — rescanner pour attribuer* signifie que le commit qui l'a
  introduit n'a pas été retrouvé ; un nouveau scan le complète.
- **ACTIF — renouveler** — la validation en direct a confirmé que la clé
  fonctionne encore.
- **Statut** et heure de **Détecté**.

Le menu en fin de ligne marque un résultat comme **Ouvert**, **Renouvelé**,
**Faux positif** ou **Ignoré**.

### Détails d'un résultat

Cliquez sur une ligne pour ouvrir le panneau de détail. Son adresse passe dans
l'URL : **Copier le lien** donne donc la même vue à un collègue ;
**Copier la référence** copie une courte référence pour un ticket. Le panneau
affiche :

- **Identifiant** — aperçu masqué, catégorie et validité. Lorsque la valeur a
  été conservée pour un propriétaire non masqué et que le masquage est
  désactivé, **Afficher** la révèle, marquée **EN CLAIR**.
- **Quota et limites** — ce que le fournisseur a renvoyé lors du test de
  validité : requêtes ou jetons restants, portées, pays, devise. Un instantané
  au moment du scan.
- **Pourquoi c'est détecté** — la règle, le fournisseur auquel il a été
  attribué, la longueur et le jeu de caractères, une variable ou un nom d'hôte
  voisin, et l'exposition. L'explication n'utilise que la forme, la longueur et
  le contexte, jamais la valeur.
- **Emplacement** — fichier, commit, auteur et date du commit, avec un lien
  vers la ligne sur GitHub.
- **Chronologie** — commit, détection, vérification et décisions de triage.
- **Que faire** — les étapes de remédiation dans l'ordre : révoquer et
  réémettre chez le fournisseur, consulter son journal d'audit, retirer la
  valeur de HEAD, décider s'il faut réécrire l'historique et ajouter une
  vérification de secrets en pre-commit.
- **Même dépôt** et **Même fournisseur** — résultats liés, à un clic.
- **Métadonnées** — tout ce que le moteur a enregistré d'autre.

### Triage groupé

Cochez les cases en début de ligne, ou la case de l'en-tête pour sélectionner
toutes les lignes de la page. Une barre apparaît en bas avec le nombre de
lignes et les quatre statuts. **Renouvelé** et **Faux positif** demandent
d'abord une confirmation, car chacun est une affirmation sur le monde
extérieur — que la clé a été révoquée, que la correspondance était erronée — et,
en masse, c'est cette affirmation qui peut clore discrètement une vraie fuite.
Ouvert et Ignoré s'appliquent immédiatement. **Exporter la sélection** exporte
uniquement les lignes cochées.

### Exporter

**Exporter** écrit les résultats actuellement chargés, en consignant dans le
fichier les filtres qui les ont produits :

- **Détections en CSV** — une ligne par résultat, pour un tableur.
- **Détections en JSON** — structuré, pour un script.
- **Rapport d'incident** — Markdown, groupé par dépôt.
- **Synthèse de direction** — une page, sans tableau.

Les exports ne contiennent que des aperçus masqués ; l'identifiant en clair
n'est jamais écrit dans un fichier, quel que soit le format.

---

## Dépôts

Chaque dépôt scanné, classé selon la gravité de son exposition ouverte.

- **Filtrer par propriétaire/nom** et **Uniquement avec des résultats
  ouverts**.
- **Trier par** risque, résultats ouverts, dernier scan ou nom.
- Colonnes : type, nombres ouverts par gravité, ouverts / total, **Risque**,
  **Prochaine étape**, **Signalé** et dernier scan.
- Cliquez sur un dépôt pour ouvrir ses résultats.

Le **Risque** va de 0 (rien d'ouvert) à 100. Les résultats ouverts sont
pondérés par gravité — un critique compte dix fois un faible — et le total
sature, si bien qu'un dépôt avec un seul critique est déjà bien classé.
Tranches : Faible, Moyen (25+), Élevé (50+), Critique (75+). Survolez pour voir
les nombres qui le composent.

La **Prochaine étape** indique la seule chose qui vaille la peine d'être
faite : *Renouveler les clés critiques*, *Prévenir le propriétaire*,
*Notification en file*, *Échec de la notification — réessayer*, *Examiner les
détections restantes* ou *Rien d'ouvert*.

### Prévenir le propriétaire

La puce **E-mails de signalement** en haut indique si l'envoi est activé ; elle
renvoie vers les Paramètres. Tant que l'envoi est activé, **Signaler** sur une
ligne met en file un signalement destiné au propriétaire du dépôt, listant ses
résultats sous forme d'aperçus masqués. La colonne **Signalé** indique ensuite
*en cours*, *envoyé*, *échec* ou *désinscrit*.

---

## Non mappés

Le backlog de secrets à haute entropie qu'aucune règle de fournisseur nommée ne
revendique, regroupés par forme. Chaque ligne affiche un **nom supposé** (tiré
de la variable ou de la clé la plus proche — parfois juste un mot voisin), un
**échantillon** masqué, la **forme** (longueur et jeu de caractères), le
**contexte** dans lequel il a été trouvé, le nombre d'**occurrences**, son
statut et la date de dernière occurrence. Une forme qui revient sans cesse est
un fournisseur pour lequel il vaut la peine d'écrire une règle. Les résultats
de ce type s'affichent comme *non mappé* jusqu'à ce qu'une règle les
revendique ; dès que c'est le cas, le scan suivant les réattribue.

---

## Fournisseurs

Chaque fournisseur que GitScanner peut détecter et attribuer, regroupé par
catégorie (IA, cloud, données, paiements, messagerie, DevOps, outils de
développement, autres), avec le nombre de règles de chacun. Cliquez sur un
fournisseur pour voir :

- **Pourquoi cette gravité** — critique signifie contrôle total ; élevé, un
  large accès à l'API ; moyen, un accès restreint ou limité à un hôte ; faible
  identifie un compte sans pouvoir agir dessus.
- **Ce qu'une fuite permet de faire** — l'impact pour cette catégorie.
- **À quoi ressemblent les clés et comment elles sont détectées**, une carte
  par règle :
  - *préfixe propre* — la valeur commence par le préfixe du fournisseur, ce qui
    constitue une preuve à lui seul ;
  - *nom de paramètre* — une valeur sans forme propre, signalée uniquement
    lorsqu'elle est affectée à un paramètre portant le nom du fournisseur ;
  - *URL de connexion* — une URL contenant l'identifiant ; une URL qui ne
    pointe que vers localhost ou un service compose est considérée comme une
    configuration de développement et écartée ;
  - *bloc PEM* — une clé privée de BEGIN à END ;
  - *structure seule* — longueurs fixes, séparateurs ou marqueurs, volontairement
    restreinte.
  Chaque carte peut afficher la structure de la clé, le motif, le texte que le
  fichier doit contenir et un mot qui doit apparaître à proximité.
- **Après une correspondance** — les valeurs de substitution (`your-…-key`,
  `xxxx`, `changeme`), les clés de documentation et de sandbox et les jeux de
  test sont écartés ; ce qui reste n'est conservé que sous forme d'empreinte et
  d'aperçu masqué.

Un fournisseur affiche sa règle la plus grave. Sur chaque résultat, la gravité
monte d'un niveau si la fuite est encore atteignable depuis HEAD, et d'un
niveau de plus pour les fournisseurs cloud et de paiement, jusqu'à critique.

---

## Analyses

Un état de tout ce qui a été scanné jusqu'ici :

- **Résultats** et **Dépôts** — totaux, ouverts, exposés / scannés.
- **Chronologie d'introduction des fuites** — résultats par mois de commit de
  l'identifiant.
- **Ouverts par gravité**, **Ouverts par fournisseur** (fournisseurs
  identifiés ainsi que la catégorie globale des non mappés), **Par type
  d'application**, **Statut du triage**.
- **Règles de détection** — les règles qui se déclenchent le plus.
- **Validation des identifiants actifs** — actifs, révoqués, aucune sonde,
  échec de vérification.
- **E-mails de signalement** — dépôts contactés, et dépôts exposés pas encore
  signalés.
- **Dépôts les plus exposés** — selon le même score de risque que Dépôts.
- **Non résolus depuis le plus longtemps** — résultats ouverts par date de
  commit.

Les chiffres portent sur toute la période ; il n'y a pas de plage de dates, car
l'API n'en propose pas. Les filtres en haut pilotent les liens vers Résultats
que la page ouvre.

---

## E-mails de signalement (Notifications)

Chaque signalement envoyé aux propriétaires de dépôts et son résultat : dépôt,
destinataire, statut (*en cours*, *envoyé*, *échec*, *désinscrit*), nombre de
résultats, date de la demande et date d'envoi. Recherchez par dépôt ou par
destinataire.

- **Voir l'e-mail** — le message en HTML et en texte brut, régénéré à partir
  des résultats tels qu'ils sont maintenant.
- **Renvoyer** — le remet en file.

---

## Paramètres

### Apparence

Enregistré dans ce navigateur uniquement.

- **Thème** — Papier (papier vieilli, bleu canard et sang-de-bœuf), Minuit
  (terminal presque noir, vert phosphore), Océan (bleu marine, cyan et bleu),
  Signal (graphite, seules les alertes sont en couleur), Minimal (sobre,
  entreprise).
- **Clair ou sombre** — Clair, Sombre, ou Système pour suivre votre système
  d'exploitation.
- **Densité** — Compact affiche plus de lignes à l'écran.
- **Animations** — Réduites désactive les animations.
- **Démarrer avec la barre latérale repliée**.
- **Réinitialiser l'apparence** — revient aux valeurs par défaut.

### Introduction

**Revoir l'introduction** rouvre la visite guidée.

### E-mails de signalement

L'interrupteur principal. Lorsqu'il est activé, Signaler envoie un signalement
au propriétaire du dépôt. Lorsqu'il est désactivé, rien n'est envoyé et tout ce
qui était déjà en file est abandonné. Il n'existe pas encore de préférences par
gravité ni de récapitulatif.

### Masquage de sécurité et propriétaires non masqués

**Activé** (par défaut) : chaque identifiant est affiché masqué.
**Désactivé** : les identifiants trouvés dans les dépôts des propriétaires
listés sous **Propriétaires non masqués** sont affichés en entier, à des fins
de test ; pour tous les autres ils restent masqués, et les e-mails et les
exports sont toujours masqués.

- N'ajoutez que des utilisateurs ou organisations GitHub qui vous
  appartiennent.
- Les valeurs sont conservées, chiffrées, à partir du scan suivant —
  rescannez les dépôts d'un propriétaire pour voir en clair les résultats
  existants.
- Retirer un propriétaire supprime tout ce qui était conservé pour lui.
- Si le serveur n'a pas de `SECRET_SEAL_KEY`, l'affichage en clair n'est pas
  disponible et rien n'est jamais conservé. Générez-en une avec
  `openssl rand -hex 32`, ajoutez-la à `.env.local` et redémarrez.

Tant que le masquage est désactivé, un avertissement rouge reste affiché sur la
page : les identifiants complets des propriétaires listés sont visibles par
quiconque peut ouvrir ce tableau de bord.

---

## Validation en direct

Lorsque l'opérateur l'active (`PROBE_SECRETS`) pour ses propres cibles,
GitScanner effectue pour chaque résultat un appel d'identité en lecture seule
afin de savoir si la clé fonctionne encore et à quel compte elle appartient :

- **Actif — renouveler** — le fournisseur l'a acceptée. Renouvelez celle-ci en
  premier.
- **Révoqué** — le fournisseur l'a rejetée ; déjà renouvelée ou révoquée.
- **Aucune sonde** — aucune vérification n'existe pour ce fournisseur.
- **Échec de vérification** — limite de requêtes ou réseau ; réessayez plus
  tard avec un nouveau scan.

Elle est désactivée par défaut, n'énumère jamais le contenu du compte, et ses
relevés de quota et de limites apparaissent dans le panneau du résultat.

---

## Gravité

- **Critique** — contrôle total : déplacer de l'argent, prendre le contrôle du
  compte, lire tout ce qu'il protège.
- **Élevé** — large accès à l'API en tant que le compte.
- **Moyen** — accès restreint ou limité à un hôte : un canal, un webhook, un
  serveur qui doit aussi être joignable.
- **Faible** — identifie un compte sans pouvoir agir dessus, comme une clé en
  mode test. Mérite tout de même d'être renouvelée.

---

## Dépannage

- **« Impossible de joindre l'API »** — vérifiez l'adresse dans la barre
  supérieure et que la stack tourne ; la pastille passe au vert quand elle
  répond.
- **Un scan reste à 0 %** — ouvrez-le et regardez l'Activité en direct.
  *En attente de la limite GitHub* se résout tout seul ; aucune activité
  signifie que les workers ne tournent pas.
- **Un nouveau scan n'a rien trouvé de nouveau** — les résultats appartiennent
  au scan précédent ; filtrez plutôt Résultats par dépôt.
- **Commit inconnu** — rescannez le dépôt pour l'attribuer.
- **Signaler est grisé** — activez les e-mails de signalement dans les
  Paramètres.
- **Afficher est absent du panneau** — le masquage est activé, le propriétaire
  n'est pas dans la liste des non masqués, le serveur n'a pas de
  `SECRET_SEAL_KEY`, ou le dépôt n'a pas été rescanné après l'ajout du
  propriétaire.
- **La liste indique n'avoir affiné que 500 résultats** — resserrez les
  filtres côté serveur (gravité, fournisseur, statut, dépôt) pour que la
  fenêtre couvre tout.
- **Résultats périmés** — supprimer une tâche ou effacer les données retire ses
  résultats ; rescannez pour les récupérer.

GitScanner est réservé aux tests de sécurité autorisés. Scannez ce qui vous
appartient ou ce que vous avez l'autorisation de tester, et informez les
propriétaires de ce que vous trouvez.
