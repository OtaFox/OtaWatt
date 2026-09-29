# OtaWatt

**OtaWatt** est un logiciel Windows léger et open source créé par **OtaFox** pour surveiller en temps réel la consommation électrique mesurée par une prise Shelly compatible.

Il a été pensé principalement pour les Gamer, l’overclocking, les benchmarks et le diagnostic de stabilité d’un PC, avec l’objectif de fournir des mesures électriques utiles tout en consommant très peu de ressources système.

## Captures d’écran

### Interface principale

![OtaWatt - Interface principale](docs/screenshots/otawatt%20image.PNG)

### Paramètres

![OtaWatt - Paramètres](docs/screenshots/otawatt%20image2.PNG)

### Mode compact

![OtaWatt - Mode compact](docs/screenshots/otawatt%20image3.PNG)

## Fonctionnalités

- Surveillance de la consommation électrique en temps réel
- Actualisation des mesures toutes les 500 ms
- Affichage de la puissance en watts
- Affichage de la tension en volts
- Affichage de l’intensité en ampères
- Affichage de la fréquence en hertz
- Affichage de la température du Shelly
- État de santé synthétique
- Détection de plusieurs anomalies électriques
- Enregistrement des mesures au format CSV
- Synchronisation régulière des données afin de limiter leur perte lors d’un plantage
- Mode compact avec opacité réduite
- Mode toujours au premier plan
- Fonctionnement dans la zone de notification Windows
- Démarrage automatique avec Windows optionnel
- Démarrage directement en mode réduit optionnel
- Configuration de l’adresse réseau du Shelly depuis l’application
- Test de connexion avec le Shelly
- Fonctionnement local sans compte cloud obligatoire
- Aucune télémétrie OtaWatt

## État de santé

OtaWatt peut signaler notamment :

- Surpuissance
- Surchauffe
- Surtension
- Sous-tension
- Surintensité
- Fréquence anormale

Lorsque rien d’anormal n’est détecté :

`État de santé : RAS`

## Enregistrement CSV

OtaWatt peut enregistrer les mesures afin de permettre leur analyse après une session de jeu, un benchmark, un overclocking ou un plantage.

Les données enregistrées comprennent notamment :

- Horodatage
- Puissance
- Tension
- Intensité
- Fréquence
- Température
- Énergie

L’enregistrement est conçu pour limiter autant que possible la perte des dernières mesures lors d’un arrêt brutal du système ou d’une application.

## Compatibilité

### Systèmes

- Windows 10 x64
- Windows 11 x64

### Matériel testé

- Shelly Plug M Gen3

D’autres appareils Shelly utilisant une interface de mesure compatible peuvent fonctionner, mais doivent être considérés comme non testés tant que leur compatibilité n’a pas été confirmée.

## Téléchargement

### GitHub

Téléchargement direct de la version Windows x64 :

- [OtaWatt 0.5.0 - EXE](https://github.com/OtaFox/OtaWatt/releases/download/v0.5.0/OtaWatt_0.5.0.exe)
- [OtaWatt 0.5.0 - ZIP portable](https://github.com/OtaFox/OtaWatt/releases/download/v0.5.0/OtaWatt_0.5.0_Windows_x64.zip)

Page de la dernière version publiée :

https://github.com/OtaFox/OtaWatt/releases/latest

### Vérification SHA-256

`OtaWatt_0.5.0.exe`

`7da8912363872fb39982bb3c10136f3284d58ddcf0d8a26bf0ccdab8bff217b7`

`OtaWatt_0.5.0_Windows_x64.zip`

`905d8e80cebe39736042f72570a504537c0a91682cb2dfa0e4dbabe3c0d50d14`

## Microsoft Store

OtaWatt est disponible gratuitement sur le Microsoft Store :

https://apps.microsoft.com/detail/9NSX64X1TB1G

## Fonctionnement local

OtaWatt communique directement avec le Shelly sur le réseau local.

Le logiciel ne nécessite pas de compte cloud pour récupérer les mesures et n’envoie pas les relevés électriques à un serveur OtaWatt.

Les fichiers CSV restent stockés localement sur l’ordinateur de l’utilisateur.

## Performances

OtaWatt est conçu pour rester léger afin de pouvoir fonctionner pendant :

- une session de jeu
- un benchmark
- un stress test
- une session d’overclocking
- un diagnostic de stabilité

L’application fonctionne avec un seul processus principal et ne nécessite pas de service Windows supplémentaire.

## Développement

OtaWatt est actuellement développé en **Go** avec une interface Windows native.

La police **Audiowide** est intégrée directement dans l’application.

## Compilation

Le dépôt contient le code source nécessaire à la compilation d’OtaWatt.

Principaux fichiers :

```text
main.go
go.mod
OtaWatt.ico
Audiowide-Regular.ttf
LICENSE-Audiowide.txt

```

## Vie privée

OtaWatt ne contient pas de système de publicité, de suivi utilisateur ou de télémétrie propre au logiciel.

Les informations nécessaires au fonctionnement sont traitées localement.

## Licence

Le code source d’OtaWatt est distribué sous licence **MIT**.

La police **Audiowide** intégrée au projet est distribuée séparément sous licence **SIL Open Font License 1.1**.

Consultez :

- `LICENSE`
- `LICENSE-Audiowide.txt`

## Version actuelle

**OtaWatt 0.5.0**

## Auteur

**Créé par OtaFox**
