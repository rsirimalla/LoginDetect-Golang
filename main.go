package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	maxminddb "github.com/oschwald/maxminddb-golang"
)

func main() {
	api := rest.NewApi()
	api.Use(rest.DefaultDevStack...)
	router, err := rest.MakeRouter(
		rest.Post("/v1", postEvent),
	)
	if err != nil {
		log.Fatal(err)
	}

	api.SetApp(router)
	log.Fatal(http.ListenAndServe(":5000", api.MakeHandler()))
}

// Event type
type Event struct {
	Username  string `json:"username"`
	IP        string `json:"ip_address"`
	UUID      string `json:"event_uuid"`
	Timestamp int    `json:"unix_timestamp"`
}

// CurrentLocation - Current Geo loccation
type CurrentLocation struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	Radius    uint16  `json:"radius"`
}

// Response - endpoint response
type Response struct {
	Location *CurrentLocation `json:"currentGeo"`
}

// PostEvent handler
func postEvent(w rest.ResponseWriter, r *rest.Request) {

	// Response
	response := Response{}

	// Decode payload to JSON
	decoder := json.NewDecoder(r.Body)
	event := Event{}
	err := decoder.Decode(&event)

	// Validate payload
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if event.UUID == "" {
		rest.Error(w, "event uuid required", 400)
		return
	}
	if event.Username == "" {
		rest.Error(w, "username required", 400)
		return
	}
	if event.IP == "" {
		rest.Error(w, "ip address required", 400)
		return
	}
	if event.Timestamp == 0 {
		rest.Error(w, "unix_timestamp required", 400)
		return
	}

	// Get Geolocation details
	setLocation(event, &response)

	w.WriteJson(&response)
}

// getLocation - get location
func setLocation(e Event, response *Response) {
	gdb, err := maxminddb.Open("./GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer gdb.Close()

	ip := net.ParseIP(e.IP)
	var record struct {
		Location struct {
			AccuracyRadius uint16  `maxminddb:"accuracy_radius"`
			Latitude       float64 `maxminddb:"latitude"`
			Longitude      float64 `maxminddb:"longitude"`
			MetroCode      uint    `maxminddb:"metro_code"`
			TimeZone       string  `maxminddb:"time_zone"`
		} `maxminddb:"location"`
	}

	err = gdb.Lookup(ip, &record)
	if err != nil {
		log.Fatal(err)
	}
	curretLocation := CurrentLocation{}
	curretLocation.Radius = record.Location.AccuracyRadius
	curretLocation.Latitude = record.Location.Latitude
	curretLocation.Longitude = record.Location.Longitude

	response.Location = &curretLocation
}
