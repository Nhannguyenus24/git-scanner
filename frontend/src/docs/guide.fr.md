# Utilisation de GitScanner

GitScanner clone l'historique complet d'un dépôt, parcourt chaque blob unique et
signale les identifiants détectés — tant dans l'arbre actuel que dans tout l'historique.
Cette page constitue le guide rapide d'utilisation du tableau de bord.

## Connexion

L'adresse de l'API est affichée dans la barre supérieure ; la pastille à côté devient verte
lorsque le tableau de bord parvient à la joindre. Pointez vers votre propre hôte d'API si vous
n'utilisez pas l'adresse par défaut `http://localhost:8080`. L'adresse, votre thème et votre
langue sont conservés dans ce navigateur.

Utilisez les commandes en haut à droite pour basculer entre le **mode clair et sombre** et
pour choisir votre **langue**.

## Démarrer un scan

Cliquez sur **Nouveau scan** (en haut à gauche ou dans la barre latérale). Choisissez l'élément à scanner :

- **Dépôt** — une URL de clonage HTTPS, par ex. `https://github.com/owner/repo`.
- **Utilisateur** ou **Organisation** — tous les dépôts appartenant à ce compte. Cochez
  *Inclure les dépôts forké (forks)* pour scanner également les forks.

Le scan s'exécute en arrière-plan. La page s'ouvre sur la vue détaillée du scan, où la barre
de progression et les compteurs se mettent à jour à mesure que les dépôts sont analysés.

## Lecture des résultats

La **Vue d’ensemble** présente les informations essentielles : résultats ouverts, nombre de critiques,
combien sont encore actifs sur HEAD, et les répartitions par gravité, fournisseur et état de triage.
Chaque tuile et chaque barre est un lien vers une liste filtrée de Résultats.

Les **Analyses** vont plus loin — une chronologie d'introduction des identifiants et des
répartitions par type d'application et par règle de détection.

**Résultats** est la liste complète et filtrable. Filtrez par gravité, fournisseur, statut
ou recherche libre ; le filtre est conservé dans l'URL, ce qui permet de partager ou de mettre
en favori une vue filtrée. Chaque ligne affiche :

- la gravité et le fournisseur auquel appartient l'identifiant ;
- un aperçu masqué — le secret réel ne quitte jamais le moteur ;
- le fichier et la ligne, avec un lien vers le fichier sur GitHub ;
- le commit qui l'a introduit, son auteur et sa date ;
- **Exposition** — `sur HEAD` signifie qu'il est toujours présent dans l'arbre actuel ; `historique`
  signifie qu'il a été retiré mais reste accessible lors du clonage ;
- un badge **Actif — renouveler** lorsque la validation en direct a confirmé que la clé fonctionne toujours.

Utilisez le menu à l'extrémité de chaque ligne pour marquer un résultat comme **Renouvelé**,
**Faux positif** ou **Ignoré** lors de leur traitement.

## Dépôts et formes non mappées

**Dépôts** classe chaque dépôt scanné selon son exposition ouverte, ventilée par gravité.
**Non mappés** rassemble les secrets à haute entropie qu'aucune règle connue n'a encore réclamés,
regroupés par forme — une forme fréquente correspond à un fournisseur méritant une règle dédiée.

## Validation en direct

Lorsque l'opérateur l'a activée pour ses propres cibles, GitScanner effectue un appel d'identité
en lecture seule par résultat afin de confirmer si la clé fonctionne encore et à quel compte elle appartient.
Une clé confirmée active est celle à renouveler en priorité. Elle est désactivée par défaut et
n'énumère jamais le contenu du compte.
