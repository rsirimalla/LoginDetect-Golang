package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	_ "github.com/mattn/go-sqlite3"
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
	Timestamp int64  `json:"unix_timestamp"`
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
	PreEvent *PreSubEvent     `json:"precedingIpAccess"`
	SubEvent *PreSubEvent     `json:"subsequentIpAccess"`
}

// PreSubEvent - Prceeding or Subsequent Event
type PreSubEvent struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	Radius    uint16  `json:"radius"`
	IP        string  `json:"ip_address"`
	// Speed     int     `json:"speed"`
	Timestamp int64 `json:"timestamp"`
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

	// Insert into DB
	insertIntoDB(event, response)

	// Set adjecent events
	setAdjEvents(event, "previous", &response)
	setAdjEvents(event, "subsequent", &response)

	// Write response
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

// insertIntoDB - Insert event into database
func insertIntoDB(e Event, r Response) {
	db, err := sql.Open("sqlite3", "./detector.db")
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := db.Prepare("insert into login_geo_location(username, event_uuid,ip_address, unix_timestamp, lat, lon, radius) values(?,?,?,?,?,?,?)")
	_, err = stmt.Exec(e.Username, e.UUID, e.IP, e.Timestamp, r.Location.Latitude, r.Location.Longitude, r.Location.Radius)
	if err != nil {
		log.Fatal(err)
	}
	stmt.Close()
	defer db.Close()
}

// setAdjEvents - set preceeding or subsequent events
func setAdjEvents(e Event, acctype string, r *Response) {
	db, err := sql.Open("sqlite3", "./detector.db")
	if err != nil {
		log.Fatal(err)
	}

	sql := "select ip_address as ipAddress, unix_timestamp as timestamp, lat, lon, radius from login_geo_location where username=? and unix_timestamp SIGN ?  order by unix_timestamp ORDER limit 1"
	if acctype == "previous" {
		sql = strings.Replace(sql, "SIGN", "<", -1)
		sql = strings.Replace(sql, "ORDER", "desc", -1)
	} else {
		sql = strings.Replace(sql, "SIGN", ">", -1)
		sql = strings.Replace(sql, "ORDER", "", -1)
	}

	fmt.Println(sql)

	var ipAddress string
	var timestamp time.Time
	var lat float64
	var lon float64
	var radius uint16

	err = db.QueryRow(sql, e.Username, e.Timestamp).Scan(&ipAddress, &timestamp, &lat, &lon, &radius)
	if err != nil {
		log.Fatal(err)
	}

	event := PreSubEvent{}
	event.IP = ipAddress
	event.Timestamp = timestamp.Unix()
	event.Radius = radius
	event.Latitude = lat
	event.Longitude = lon

	if acctype == "previous" {
		r.PreEvent = &event
	} else {
		r.SubEvent = &event
	}
	defer db.Close()
}
