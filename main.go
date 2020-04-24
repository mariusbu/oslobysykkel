package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// We're using the open API from Oslo Bysykkel
// See https://oslobysykkel.no/apne-data/sanntid

const (
	requestTimeout            = 10 * time.Second
	clientIdentifier          = "test-test"
	stationInformationAddress = "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json"
	stationStatusAddress      = "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json"
)

var (
	client *http.Client
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

func fetchData() (map[string]gbfsStationInformationStation, map[string]gbfsStationStatusStation, error) {

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

	return informationMap, statusMap, err
}

func main() {
	client = &http.Client{Timeout: requestTimeout}

	info, status, err := fetchData()
	if err != nil {
		fmt.Println(err)
	}

	for _, inf := range info {
		fmt.Println(inf)
	}

	for _, stat := range status {
		fmt.Println(stat)
	}
}
