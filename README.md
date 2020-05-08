# Oslo Bysykkel test

[Go](https://golang.org/) kommandoline-applikasjon som viser en oversikt over Oslo Bysykkel stasjoner med tilgjengelige låser og ledige sykler.

## Kjøre programmet

Den eneste avhengigheten utover standard Go pakker er [tview](github.com/rivo/tview).
Den lastes ned med

`go get github.com/rivo/tview`

Programmet kjøres med

`go run main.go`

## Kjøre testene

Enhetstestene kjøres med

`go test`

## Hvem er brukeren ? 

Dette blir selvsagt litt kunstig, gitt utgangspunktet. Men jeg har bestemt meg for at brukeren er noen som lever i kommandolinjen (f.eks. en utvikler eller sysadmin), og som har lyst til å følge med på om det er ledige sykler ved hans/hennes/hens favoritt-stasjon. 🤓

## Forbedringer

1. I et større prosjekt ville jeg organisert koden i pakker som implementerer spesifik funksjonalitet (f.eks. GBFS), men for enkelhets skyld har jeg beholdt all koden i én fil.
1. Jeg vurderte om jeg burde gå via auto-discovery for å finne API-endepunktene som brukes, men valgte å ikke å gjøre det. Dette var litt for å holde det enkelt og litt fordi GBFS ikke sier noe om at de andre API-endepunktene kan endre seg. 🙈
1. Tilbakemelding fra 100% av test-brukerne er at hun ønsket å kunne søke etter sin favoritt-stasjon. 😛