# Oslo Bysykkel test

[Go](https://golang.org/) api-server som gir en liste med Oslo Bysykkel stasjoner med tilgjengelige låser og ledige sykler.

## Kjøre programmet

Vi har to avhengighetener utover standard Go pakker; [Gorilla Mux](github.com/gorilla/mux) og [go-cache](github.com/patrickmn/go-cache).
De lastes ned med

```
go get -u github.com/gorilla/mux
go get -u github.com/patrickmn/go-cache
```

Serveren kjøres med

`go run main.go`

Vi kan sjekke at serveren kjører ved å kalle

`curl http://localhost:8080`

## Kjøre testene

Enhetstestene kjøres med

`go test`

## Bruk av API'et

Listen over stasjoner kan hentes med

`curl http://localhost:8080/api/v1/stations`

Det returnerer en JSON matrise (array) som ser slik ut

```
[
    {
        "station_id":"496",
        "name":"Fysikkbygningen",
        "num_bikes_available":2,
        "num_docks_available":43
    },
    {
        "station_id":"574",
        "name":"Annette Thommessens Plass",
        "num_bikes_available":12,
        "num_docks_available":0
    }
]
```

En enkelt stasjon kan hentes med

`curl http://localhost:8080/api/v1/stations/<station_id>`

Det returnerer et JSON objekt som ser slik ut

```
{
    "station_id":"610",
    "name":"Sotahjørnet",
    "num_bikes_available":0,
    "num_docks_available":20
}
```

## Kommentarer

1. I et større prosjekt ville jeg organisert koden i pakker som implementerer spesifik funksjonalitet (f.eks. GBFS), men for enkelhets skyld har jeg beholdt all koden i én fil.
1. Jeg vurderte om jeg burde gå via auto-discovery for å finne API-endepunktene som brukes, men valgte å ikke å gjøre det. Dette var litt for å holde det enkelt og litt fordi GBFS ikke sier noe om at de andre API-endepunktene kan endre seg. 🙈
1. For et større prosjekt ville jeg vurdert et verktøy for å generere REST API dokumentasjon (f.eks. Swagger).
1. Serveren hører på port 8080 for å gjøre det enklere å teste uten å endre system-konfigurasjoner for å tillate å binde til port 80.
1. Serveren bruker ikke TLS (https) fordi det krever et ssl-sertifikat. Jeg antar at det ikke er nødvendig for denne test-serveren.
1. Vi kunne autentisert kall til API'et vårt med en nøkkel (f.eks. JWT). Men siden Oslo Bysykkel heller ikke bruker dette, har jeg valgt å ikke implementere dette.
1. Vi kunne sjekke om klienten spør om data på et annet format ved å se på Accept-feltet i header'en. Vi kunne f.eks. også returnere XML hvis klienten ønsker det.
1. Vi kunne legge til støtte for sider (paging) på `api/v1/stations` endepunktet.
1. Jeg har brukt URI-versjonering av API'et, men et alternativ kunne vært Media Type versjonering.
1. Oslo BySykkel API'et ser ut til å støtte etags, så vi kunne bruke det for å redusere data-mengden vi henter.
1. Serveren kjører en egen rutine som henter data fra Oslo BySykkel API'et med gjevne mellomrom og legger det i en cache.
Fordelen med dette er at det gjør implementasjonen av serveren relativt enkel fordi våre endepunkter kan returnere data rett fra cache.
Ulempen er at vi spør om data fra Oslo BySykkel API'et selv om det er få eller ingen forespørsler til vår egen server.