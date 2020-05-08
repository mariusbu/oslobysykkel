package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

type testFetchCase struct {
	ResponseStatusCode     int
	ResponseBody           string
	ExpectedRequestAddress string
	ExpectError            bool
	ExpectedBody           []byte
}

type testFetchStationInformationCase struct {
	testFetchCase
	ExpectedInformation gbfsStationInformation
}

type testFetchStationStatusCase struct {
	testFetchCase
	ExpectedStatus gbfsStationStatus
}

type testFetchDataCase struct {
	FetchStatus      testFetchCase
	FetchInformation testFetchCase
	ExpectedData     []stationData
}

// NOTE: rather than spinning up a httptest.Server we test by replacing the
// default http.Transport with our own http.RoundTripper implementation.
// This also allows us to check the URLs we are using without hitting the
// BySykkel API servers.

type CustomTransport func(req *http.Request) *http.Response

func (ct CustomTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return ct(request), nil
}

func verifyFetchRequest(t *testing.T, expectedURL string, request *http.Request) {

	const expectedHTTPMethod = http.MethodGet
	if request.Method != expectedHTTPMethod {
		t.Errorf("The request Method `%s` is different from the expected `%s`", request.Method, expectedHTTPMethod)
	}

	const expectedClientIdentifier = "test-test"
	if request.Header.Get("Client-Identifier") != expectedClientIdentifier {
		t.Errorf("The request Client-Identifier `%s` is different from the expected `%s`", request.Header.Get("Client-Identifier"), expectedClientIdentifier)
	}

	if request.URL.String() != expectedURL {
		t.Errorf("The request URL `%s` is different from the expected `%s`", request.URL.String(), expectedURL)
	}
}

// We currently test the following scenarios (thought to be most likely to affect the user):
//
// Happy path            - the server returns status code 200, and the data we expect
// Empty response body   - the server returns status code 200, but the body is empty
// Garbled response body - the server returns status code 200, but the body us garbled
// Internal Server Error - the server returns status code != 200 (in this case 500),
//                         and the body contains an error message

func TestFetchBase(t *testing.T) {

	stationInformationResponse, err := ioutil.ReadFile("main_testdata/station_information.json")
	if err != nil {
		t.Errorf("Failed to read the test data file: %s", err.Error())
	}

	testCases := []testFetchCase{
		{
			// Happy path
			ResponseStatusCode:     http.StatusOK,
			ResponseBody:           string(stationInformationResponse),
			ExpectedRequestAddress: "https://hostname.com/path/to",
			ExpectError:            false,
			ExpectedBody:           stationInformationResponse,
		},
		{
			// Empty response body
			ResponseStatusCode:     http.StatusOK,
			ResponseBody:           ``,
			ExpectedRequestAddress: "https://hostname.com/path/to",
			ExpectError:            false,
			ExpectedBody:           []byte(``),
		},
		{
			// Garbled response body
			ResponseStatusCode:     http.StatusOK,
			ResponseBody:           `{#$`,
			ExpectedRequestAddress: "https://hostname.com/path/to",
			ExpectError:            false,
			ExpectedBody:           []byte(`{#$`),
		},
		{
			// Internal Server Error
			ResponseStatusCode:     http.StatusInternalServerError,
			ResponseBody:           `Internal Server Error`,
			ExpectedRequestAddress: "https://hostname.com/path/to",
			ExpectError:            true,
			ExpectedBody:           nil,
		},
	}

	for _, testCase := range testCases {

		client = &http.Client{Transport: CustomTransport(func(request *http.Request) *http.Response {
			verifyFetchRequest(t, testCase.ExpectedRequestAddress, request)
			return &http.Response{
				StatusCode: testCase.ResponseStatusCode,
				Body:       ioutil.NopCloser(bytes.NewBufferString(testCase.ResponseBody)),
				Header:     make(http.Header),
			}
		})}

		body, err := fetch("https://hostname.com/path/to")

		if !testCase.ExpectError && err != nil {
			t.Errorf("We got an unexpected error: %s", err.Error())
		}

		if testCase.ExpectError && err == nil {
			t.Errorf("We did not receive the expected error")
		}

		if !reflect.DeepEqual(body, testCase.ExpectedBody) {
			t.Errorf("The received body data is different from the expected body data")
		}
	}
}

func TestFetchStationInformation(t *testing.T) {

	stationInformationResponse, err := ioutil.ReadFile("main_testdata/station_information.json")
	if err != nil {
		t.Errorf("Failed to read the test data file: %s", err.Error())
	}

	testCases := []testFetchStationInformationCase{
		{
			// Happy path
			testFetchCase: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationInformationResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            false,
			},
			ExpectedInformation: gbfsStationInformation{
				LastUpdated: 1553592653,
				Data: gbfsStationInformationData{
					Stations: []gbfsStationInformationStation{
						{
							StationID: "627",
							Name:      "Skøyen Stasjon",
							Address:   "Skøyen Stasjon",
							Latitude:  59.9226729,
							Longitude: 10.6788129,
							Capacity:  20,
						},
						{
							StationID: "623",
							Name:      "7 Juni Plassen",
							Address:   "7 Juni Plassen",
							Latitude:  59.9150596,
							Longitude: 10.7312715,
							Capacity:  15,
						},
						{
							StationID: "610",
							Name:      "Sotahjørnet",
							Address:   "Sotahjørnet",
							Latitude:  59.9099822,
							Longitude: 10.7914482,
							Capacity:  20,
						},
					},
				},
			},
		},
		{
			// Empty response data
			testFetchCase: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           ``,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedInformation: gbfsStationInformation{},
		},
		{
			// Garbled response data
			testFetchCase: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           `{$#`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedInformation: gbfsStationInformation{},
		},
		{
			// Internal Server Error
			testFetchCase: testFetchCase{
				ResponseStatusCode:     http.StatusInternalServerError,
				ResponseBody:           `Internal Server Error`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedInformation: gbfsStationInformation{},
		},
	}

	for _, testCase := range testCases {

		client = &http.Client{Transport: CustomTransport(func(request *http.Request) *http.Response {
			verifyFetchRequest(t, testCase.ExpectedRequestAddress, request)
			return &http.Response{
				StatusCode: testCase.ResponseStatusCode,
				Body:       ioutil.NopCloser(bytes.NewBufferString(testCase.ResponseBody)),
				Header:     make(http.Header),
			}
		})}

		informationChannel := make(chan stationInformationResult)
		defer close(informationChannel)

		go fetchStationInformation(informationChannel)

		informationResult := <-informationChannel

		if !testCase.ExpectError && informationResult.Error != nil {
			t.Errorf("We got an unexpected error: %s", informationResult.Error.Error())
		}

		if testCase.ExpectError && informationResult.Error == nil {
			t.Errorf("We did not receive the expected error")
		}

		if !reflect.DeepEqual(informationResult.Information, testCase.ExpectedInformation) {
			t.Errorf("The received station information is different from the expected station information")
		}
	}
}

func TestFetchStationStatus(t *testing.T) {
	stationStatusResponse, err := ioutil.ReadFile("main_testdata/station_status.json")
	if err != nil {
		t.Errorf("Failed to read the test data file: %s", err.Error())
	}

	testCases := []testFetchStationStatusCase{
		{
			// Happy path
			testFetchCase: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationStatusResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            false,
			},
			ExpectedStatus: gbfsStationStatus{
				LastUpdated: 1540219230,
				Data: gbfsStationStatusData{
					Stations: []gbfsStationStatusStation{
						{
							StationID:              "627",
							NumberOfBikesAvailable: 7,
							NumberOfDocksAvailable: 5,
							IsInstalled:            1,
							IsRenting:              1,
							IsReturning:            1,
							LastReported:           1540219230,
						},
						{
							StationID:              "623",
							NumberOfBikesAvailable: 4,
							NumberOfDocksAvailable: 8,
							IsInstalled:            1,
							IsRenting:              1,
							IsReturning:            1,
							LastReported:           1540219230,
						},
						{
							StationID:              "610",
							NumberOfBikesAvailable: 4,
							NumberOfDocksAvailable: 9,
							IsInstalled:            1,
							IsRenting:              1,
							IsReturning:            1,
							LastReported:           1540219230,
						},
					},
				},
			},
		},
		{
			// Empty response data
			testFetchCase: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           ``,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			ExpectedStatus: gbfsStationStatus{},
		},
		{
			// Garbled response data
			testFetchCase: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           `{$#`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			ExpectedStatus: gbfsStationStatus{},
		},
		{
			// Internal Server Error
			testFetchCase: testFetchCase{
				ResponseStatusCode:     http.StatusInternalServerError,
				ResponseBody:           `Internal Server Error`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			ExpectedStatus: gbfsStationStatus{},
		},
	}

	for _, testCase := range testCases {

		client = &http.Client{Transport: CustomTransport(func(request *http.Request) *http.Response {
			verifyFetchRequest(t, testCase.ExpectedRequestAddress, request)
			return &http.Response{
				StatusCode: testCase.ResponseStatusCode,
				Body:       ioutil.NopCloser(bytes.NewBufferString(testCase.ResponseBody)),
				Header:     make(http.Header),
			}
		})}

		statusChannel := make(chan stationStatusResult)
		defer close(statusChannel)

		go fetchStationStatus(statusChannel)

		statusResult := <-statusChannel

		if !testCase.ExpectError && statusResult.Error != nil {
			t.Errorf("We got an unexpected error: %s", statusResult.Error.Error())
		}

		if testCase.ExpectError && statusResult.Error == nil {
			t.Errorf("We did not receive the expected error")
		}

		if !reflect.DeepEqual(statusResult.Status, testCase.ExpectedStatus) {
			t.Errorf("The received station status is different from the expected station status")
		}
	}
}

func TestFetchData(t *testing.T) {

	stationInformationResponse, err := ioutil.ReadFile("main_testdata/station_information.json")
	if err != nil {
		t.Errorf("Failed to read the test data file: %s", err.Error())
	}

	stationStatusResponse, err := ioutil.ReadFile("main_testdata/station_status.json")
	if err != nil {
		t.Errorf("Failed to read the test data file: %s", err.Error())
	}

	testCases := []testFetchDataCase{
		{
			// Happy path
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationStatusResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            false,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationInformationResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            false,
			},
			ExpectedData: []stationData{
				{
					Name:                   "7 Juni Plassen",
					NumberOfBikesAvailable: 4,
					NumberOfDocksAvailable: 8,
				},
				{
					Name:                   "Skøyen Stasjon",
					NumberOfBikesAvailable: 7,
					NumberOfDocksAvailable: 5,
				},
				{
					Name:                   "Sotahjørnet",
					NumberOfBikesAvailable: 4,
					NumberOfDocksAvailable: 9,
				},
			},
		},
		{
			// Empty station status response data
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           ``,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationInformationResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            false,
			},
			ExpectedData: nil,
		},
		{
			// Empty station information response data
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationStatusResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            false,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           ``,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedData: nil,
		},
		{
			// Empty station status and information response data
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           ``,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           ``,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedData: nil,
		},
		{
			// Garbled station status response data
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           `{#$`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationInformationResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            false,
			},
			ExpectedData: nil,
		},
		{
			// Garbled station information response data
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationStatusResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            false,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           `{#$`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedData: nil,
		},
		{
			// Garbled station status and information response data
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           `{#$`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           `{#$`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedData: nil,
		},
		{
			// Internal Server Error station status response
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusInternalServerError,
				ResponseBody:           `Internal Server Error`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationInformationResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            false,
			},
			ExpectedData: nil,
		},
		{
			// Internal Server Error station information response
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusOK,
				ResponseBody:           string(stationStatusResponse),
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            false,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusInternalServerError,
				ResponseBody:           `Internal Server Error`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedData: nil,
		},
		{
			// Internal Server Error for both station status and information responses
			FetchStatus: testFetchCase{
				ResponseStatusCode:     http.StatusInternalServerError,
				ResponseBody:           `Internal Server Error`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_status.json",
				ExpectError:            true,
			},
			FetchInformation: testFetchCase{
				ResponseStatusCode:     http.StatusInternalServerError,
				ResponseBody:           `Internal Server Error`,
				ExpectedRequestAddress: "https://gbfs.urbansharing.com/oslobysykkel.no/station_information.json",
				ExpectError:            true,
			},
			ExpectedData: nil,
		},
	}

	for _, testCase := range testCases {

		client = &http.Client{Transport: CustomTransport(func(request *http.Request) *http.Response {

			switch request.URL.String() {

			case testCase.FetchStatus.ExpectedRequestAddress:
				verifyFetchRequest(t, testCase.FetchStatus.ExpectedRequestAddress, request)
				return &http.Response{
					StatusCode: testCase.FetchStatus.ResponseStatusCode,
					Body:       ioutil.NopCloser(bytes.NewBufferString(testCase.FetchStatus.ResponseBody)),
					Header:     make(http.Header),
				}

			case testCase.FetchInformation.ExpectedRequestAddress:
				verifyFetchRequest(t, testCase.FetchInformation.ExpectedRequestAddress, request)
				return &http.Response{
					StatusCode: testCase.FetchInformation.ResponseStatusCode,
					Body:       ioutil.NopCloser(bytes.NewBufferString(testCase.FetchInformation.ResponseBody)),
					Header:     make(http.Header),
				}

			default:
				t.Errorf("The request URL `%s` did not match any of the expected URLs", request.URL.String())

			}

			return &http.Response{}
		})}

		stations, _, err := fetchData()

		if !testCase.FetchStatus.ExpectError && !testCase.FetchInformation.ExpectError && err != nil {
			t.Errorf("We got an unexpected error: %s", err.Error())
		}

		if (testCase.FetchStatus.ExpectError || testCase.FetchInformation.ExpectError) && err == nil {
			t.Errorf("We did not receive the expected error")
		}

		if !reflect.DeepEqual(stations, testCase.ExpectedData) {
			t.Errorf("The received stations data is different from the expected stations data")
		}
	}
}
