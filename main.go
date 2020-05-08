package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"time"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

// We're using the open API from Oslo Bysykkel
// See https://oslobysykkel.no/apne-data/sanntid

const (
	updateInterval            = 10 * time.Second
	requestTimeout            = 10 * time.Second
	clientIdentifier          = "test-test"
	stationInformationAddress = "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json"
	stationStatusAddress      = "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json"
)

var (
	client *http.Client
	app    *tview.Application
	frame  *tview.Frame
	table  *tview.Table
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
	Name                   string
	NumberOfBikesAvailable int
	NumberOfDocksAvailable int
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Http GET to %s failed with status code %d", url, resp.StatusCode)
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

func fetchData() ([]stationData, string, error) {

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
		return nil, " 🚒 Vi klarte ikke å hente data. Vent litt, så prøver vi igjen!", err
	}

	// NOTE: we assume that having more status elements than information elements is not a problem.
	// Missing status for a station will also not result in an error, but we will inform the user.

	var message string
	stations := make([]stationData, 0, len(informationMap))
	for stationID, information := range informationMap {
		status, exists := statusMap[stationID]
		if !exists {
			message = " 🙈 Vi mangler status for noen stasjoner. Vent litt, så prøver vi igjen!"
		} else {
			stations = append(stations, stationData{
				Name:                   information.Name,
				NumberOfDocksAvailable: status.NumberOfDocksAvailable,
				NumberOfBikesAvailable: status.NumberOfBikesAvailable,
			})
		}
	}

	sort.Slice(stations, func(i, j int) bool { return stations[i].Name < stations[j].Name })

	return stations, message, err
}

func updateTable() {
	for {
		stations, message, err := fetchData()

		app.QueueUpdateDraw(func() {
			offsetRow, offsetColumn := table.GetOffset()

			if err == nil {
				table.Clear()
				table.SetCell(0, 0, &tview.TableCell{Text: " Stasjon ", Align: tview.AlignCenter, Color: tcell.ColorLightBlue})
				table.SetCell(0, 1, &tview.TableCell{Text: " Tilgjengelige låser ", Align: tview.AlignCenter, Color: tcell.ColorLightBlue})
				table.SetCell(0, 2, &tview.TableCell{Text: " Ledige sykler ", Align: tview.AlignCenter, Color: tcell.ColorLightBlue})

				for row, station := range stations {
					bikes := fmt.Sprintf("%d", station.NumberOfBikesAvailable)
					docks := fmt.Sprintf("%d", station.NumberOfDocksAvailable)
					table.SetCell(row+1, 0, &tview.TableCell{Text: station.Name, Align: tview.AlignLeft, Color: tcell.ColorWhite})
					table.SetCell(row+1, 1, &tview.TableCell{Text: bikes, Align: tview.AlignCenter, Color: tcell.ColorWhite})
					table.SetCell(row+1, 2, &tview.TableCell{Text: docks, Align: tview.AlignCenter, Color: tcell.ColorWhite})
				}
				table.SetOffset(offsetRow, offsetColumn)
			}

			updateFrameTexts(message)
		})

		time.Sleep(updateInterval)
	}
}

func updateFrameTexts(message string) {
	frame.Clear()
	frame.AddText(" 🚴 Oslo BySykkel 🚴", true, tview.AlignLeft, tcell.ColorLightBlue).
		AddText("", true, tview.AlignLeft, tcell.ColorLightBlue).
		AddText(" Hei!👋\t Du kan bla i listen med ⍐ og ⍗. Avslutt med 'q'.", true, tview.AlignLeft, tcell.ColorLightBlue).
		AddText(message, false, tview.AlignLeft, tcell.ColorLightBlue)
}

func main() {
	client = &http.Client{Timeout: requestTimeout}

	table = tview.NewTable().
		SetFixed(1, 0).
		SetSeparator(tview.BoxDrawingsLightVertical).
		SetBordersColor(tcell.ColorGray).
		SetBorders(true)

	frame = tview.NewFrame(table).
		SetBorders(1, 1, 1, 1, 2, 2)

	updateFrameTexts("📦 henter data ...")

	app = tview.NewApplication().
		SetRoot(frame, true).
		SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Rune() == 'q' {
				app.Stop()
			}
			return event
		})

	go updateTable()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
