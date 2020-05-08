package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
)

// We're using the open API from Oslo Bysykkel
// See https://oslobysykkel.no/apne-data/sanntid

// NOTE: this uses port 8080 to allow testing locally without
// elevated privileges required to bind to port 80 (":http").
// For production we should use TLS and port 443 (":https").

const (
	updateInterval            = 10 * time.Second
	requestTimeout            = 10 * time.Second
	cacheCleanupInterval      = 1 * time.Minute
	cacheKey                  = "stations"
	clientIdentifier          = "test-test"
	stationInformationAddress = "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json"
	stationStatusAddress      = "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json"
	serverAddressPort         = ":8080"
)

var (
	client        *http.Client
	stationsCache *cache.Cache
)

// The 'gbfs' structures are mapped from the General Bikeshare Feed Specification
// See https://github.com/NABSA/gbfs/blob/master/gbfs.md
// Only structures relevant for us are mapped here, not the entire spec ;)
// If we were to support other providers, we could consider creating a 'gbfs' package
// that would implement the spec with all the related structures and functions.

type gbfsStationInformationStation struct {
	StationID string  `json:"station_id"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	Capacity  int     `json:"capacity"`
}

type gbfsStationInformationData struct {
	Stations []gbfsStationInformationStation `json: "stations"`
}

type gbfsStationInformation struct {
	LastUpdated int64                      `json:"last_updated"`
	Data        gbfsStationInformationData `json:"data"`
}

type gbfsStationStatusStation struct {
	StationID              string `json:"station_id"`
	NumberOfBikesAvailable int    `json:"num_bikes_available"`
	NumberOfBikesDisabled  int    `json:"num_bikes_disabled"`
	NumberOfDocksAvailable int    `json:"num_docks_available"`
	NumberOfDocksDisabled  int    `json:"num_docks_disabled"`
	IsInstalled            int    `json:"is_installed"` // NOTE: the GBFS spec says these fields
	IsRenting              int    `json:"is_renting"`   // should be booleans, but the Oslo Bysykkel
	IsReturning            int    `json:"is_returning"` // API return them as int.
	LastReported           int64  `json:"last_reported"`
}

type gbfsStationStatusData struct {
	Stations []gbfsStationStatusStation `json:"stations"`
}

type gbfsStationStatus struct {
	LastUpdated int64                 `json:"last_updated"`
	Data        gbfsStationStatusData `json:"data"`
}

type stationInformationResult struct {
	Information gbfsStationInformation
	Error       error
}

type stationStatusResult struct {
	Status gbfsStationStatus
	Error  error
}

type stationData struct {
	StationID              string `json:"station_id"`
	Name                   string `json:"name"`
	NumberOfBikesAvailable int    `json:"num_bikes_available"`
	NumberOfDocksAvailable int    `json:"num_docks_available"`
}

func fetch(url string) ([]byte, error) {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Client-Identifier", clientIdentifier)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Http GET to %s failed with status code %d", url, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func fetchStationInformation(informationChannel chan stationInformationResult) {

	body, err := fetch(stationInformationAddress)
	if err != nil {
		informationChannel <- stationInformationResult{Error: err}
		return
	}

	var stationInformation gbfsStationInformation
	err = json.Unmarshal(body, &stationInformation)
	if err != nil {
		informationChannel <- stationInformationResult{Error: err}
		return
	}

	informationChannel <- stationInformationResult{Information: stationInformation}
}

func fetchStationStatus(statusChannel chan stationStatusResult) {

	body, err := fetch(stationStatusAddress)
	if err != nil {
		statusChannel <- stationStatusResult{Error: err}
		return
	}

	var stationStatus gbfsStationStatus
	err = json.Unmarshal(body, &stationStatus)
	if err != nil {
		statusChannel <- stationStatusResult{Error: err}
		return
	}

	statusChannel <- stationStatusResult{Status: stationStatus}
}

func fetchData() (map[string]stationData, error) {

	statusChannel := make(chan stationStatusResult)
	informationChannel := make(chan stationInformationResult)

	defer close(statusChannel)
	defer close(informationChannel)

	go fetchStationStatus(statusChannel)
	go fetchStationInformation(informationChannel)

	informationMap := make(map[string]gbfsStationInformationStation)
	statusMap := make(map[string]gbfsStationStatusStation)

	var err error

	// Wait for both fetch operations to finish before we process the data
	for i := 0; i < 2; i++ {
		select {
		case statusResult := <-statusChannel:
			if statusResult.Error != nil {
				err = statusResult.Error
			} else {
				for _, station := range statusResult.Status.Data.Stations {
					statusMap[station.StationID] = station
				}
			}
		case informationResult := <-informationChannel:
			if informationResult.Error != nil {
				err = informationResult.Error
			} else {
				for _, station := range informationResult.Information.Data.Stations {
					informationMap[station.StationID] = station
				}
			}
		}
	}

	if err != nil {
		return nil, err
	}

	// NOTE: we assume that having more status elements than information elements is not a problem.
	// Missing status for a station will also not result in an error, but we will log it as a warning.

	stations := make(map[string]stationData)
	for stationID, information := range informationMap {
		status, exists := statusMap[stationID]
		if !exists {
			log.Printf("We are missing the status for some stations.")
		} else {
			stations[stationID] = stationData{
				StationID:              stationID,
				Name:                   information.Name,
				NumberOfDocksAvailable: status.NumberOfDocksAvailable,
				NumberOfBikesAvailable: status.NumberOfBikesAvailable,
			}
		}
	}

	return stations, err
}

func updateStationsCache() {

	// NOTE: we run a continous go-routine that polls the BySykkel API periodically.
	// This allows our API endpoints to return data from the cache very quickly and
	// without locking or waiting for requests to the BySykkel API.
	// The downside is that we continue to fetch data even if we have very few requests.

	for {
		log.Printf("Fetching data from the BySykkel API.")

		stations, err := fetchData()
		if err != nil {
			log.Printf("Fetching data failed with the error: %s", err.Error())
		} else {
			stationsCache.Set(cacheKey, stations, cache.DefaultExpiration)
		}

		time.Sleep(updateInterval)
	}
}

// Root implements the `/` endpoint.
// Respons with a text to indicate that the server is alive.
func Root(w http.ResponseWriter, req *http.Request) {
	// We set the Cache-Control to no-store so we can use this endpoint to check if the server is running.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "I am listening... on %s 🤖\n", serverAddressPort)
}

// AllStations implements the `stations` endpoint.
// Responds with a JSON array of all stationData objects.
func AllStations(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=10")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var cachedStations map[string]stationData
	if item, found := stationsCache.Get(cacheKey); found {
		cachedStations = item.(map[string]stationData)
	} else {
		// If the cache is empty, something went wrong
		log.Printf("The cache failed when serving `%s`.", req.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	stations := make([]stationData, 0, len(cachedStations))
	for _, station := range cachedStations {
		stations = append(stations, station)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stations)
}

// SingleStation implements the `stations/<station_id>` endpoint.
// Responds with a single JSON stationData object.
func SingleStation(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=10")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var cachedStations map[string]stationData
	if item, found := stationsCache.Get(cacheKey); found {
		cachedStations = item.(map[string]stationData)
	} else {
		// If the cache is empty, something went wrong
		log.Printf("The cache failed when serving `%s`.", req.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	stationID := mux.Vars(req)["id"]
	if station, ok := cachedStations[stationID]; ok {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(station)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func main() {
	client = &http.Client{Timeout: requestTimeout}
	stationsCache = cache.New(cache.NoExpiration, cacheCleanupInterval)

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", Root)
	router.HandleFunc("/api/v1/stations", AllStations).Methods("GET")
	router.HandleFunc("/api/v1/stations/{id}", SingleStation).Methods("GET")

	go updateStationsCache()

	log.Printf("Starting server on %s", serverAddressPort)
	log.Fatal(http.ListenAndServe(serverAddressPort, router))
}
