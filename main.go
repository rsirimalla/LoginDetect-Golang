package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math"
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
	Location    *CurrentLocation `json:"currentGeo"`
	PreEvent    *PreSubEvent     `json:"precedingIpAccess"`
	SubEvent    *PreSubEvent     `json:"subsequentIpAccess"`
	ToCurrent   bool             `json:"travelToCurrentGeoSuspicious"`
	FromCurrent bool             `json:"travelFromCurrentGeoSuspicious"`
}

// PreSubEvent - Prceeding or Subsequent Event
type PreSubEvent struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	Radius    uint16  `json:"radius"`
	IP        string  `json:"ip_address"`
	Speed     float64 `json:"speed"`
	Timestamp int64   `json:"timestamp"`
}

// PostEvent handler
func postEvent(w rest.ResponseWriter, r *rest.Request) {

	// Speed threshold (miles/hr)
	speedThreshold := 500.0
	// Response
	response := Response{}

	// Decode payload to JSON
	decoder := json.NewDecoder(r.Body)
	event := Event{}
	err := decoder.Decode(&event)

	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Validate payload
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
	err = insertIntoDB(event, response)
	if err != nil {
		rest.Error(w, err.Error(), 400)
		return
	}

	// Set adjecent events
	err = setAdjEvents(event, "previous", &response)
	if err != nil {
		rest.Error(w, err.Error(), 400)
		return
	}
	err = setAdjEvents(event, "subsequent", &response)
	if err != nil {
		rest.Error(w, err.Error(), 400)
		return
	}

	// Set suspicious flags
	response.ToCurrent = response.PreEvent.Speed > speedThreshold
	response.FromCurrent = response.SubEvent.Speed > speedThreshold

	// Write response
	w.WriteJson(&response)
}

func validation(e Event) {

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
func insertIntoDB(e Event, r Response) error {
	db, err := sql.Open("sqlite3", "./detector.db")
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := db.Prepare("insert into login_geo_location(username, event_uuid,ip_address, unix_timestamp, lat, lon, radius) values(?,?,?,?,?,?,?)")
	_, err = stmt.Exec(e.Username, e.UUID, e.IP, e.Timestamp, r.Location.Latitude, r.Location.Longitude, r.Location.Radius)
	if err != nil {
		log.Println(err)
		return errors.New("Unable to insert into DB")
	}
	stmt.Close()
	defer db.Close()
	return err
}

// setAdjEvents - set preceeding or subsequent events
func setAdjEvents(e Event, acctype string, r *Response) error {

	// Adjecent event
	event := PreSubEvent{}
	// Set preceeding/subsequent property based on access type
	if acctype == "previous" {
		r.PreEvent = &event
	} else {
		r.SubEvent = &event
	}

	// Get DB connection
	db, err := sql.Open("sqlite3", "./detector.db")
	if err != nil {
		return errors.New(err.Error())
	}

	// Get data from database
	sql := "select ip_address as ipAddress, unix_timestamp as timestamp, lat, lon, radius from login_geo_location where username=? and unix_timestamp SIGN ?  order by unix_timestamp ORDER limit 1"
	if acctype == "previous" {
		sql = strings.Replace(sql, "SIGN", "<", -1)
		sql = strings.Replace(sql, "ORDER", "desc", -1)
	} else {
		sql = strings.Replace(sql, "SIGN", ">", -1)
		sql = strings.Replace(sql, "ORDER", "", -1)
	}

	// Define db columns
	var ipAddress string
	var timestamp time.Time
	var lat float64
	var lon float64
	var radius uint16
	err = db.QueryRow(sql, e.Username, e.Timestamp).Scan(&ipAddress, &timestamp, &lat, &lon, &radius)
	if err == nil {
		// Set properties for adjecent event
		event.IP = ipAddress
		event.Timestamp = timestamp.Unix()
		event.Radius = radius
		event.Latitude = lat
		event.Longitude = lon

		// Calculate speed
		distance := Distance(event.Latitude, event.Longitude, r.Location.Latitude, r.Location.Longitude)
		distance = distance / 1000                                             //Convert to KM
		distance = distance + float64(event.Radius+r.Location.Radius)          // Distance with uncertainity
		distance = distance * 0.625                                            //Convert to miles
		timeDelta := math.Abs(float64(event.Timestamp) - float64(e.Timestamp)) // delta between timestamps
		speed := (distance / timeDelta) * 3600                                 // miles/hr
		event.Speed = math.Round(speed)
	}

	// Close DB connection
	defer db.Close()
	return nil
}

// Distance function returns the distance (in meters) between two points of
//     a given longitude and latitude relatively accurately (using a spherical
//     approximation of the Earth) through the Haversin Distance Formula for
//     great arc distance on a sphere with accuracy for small distances
//
// point coordinates are supplied in degrees and converted into rad. in the func
//
// distance returned is METERS!!!!!!
// http://en.wikipedia.org/wiki/Haversine_formula
func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	// convert to radians
	// must cast radius as float to multiply later
	var la1, lo1, la2, lo2, r float64
	la1 = lat1 * math.Pi / 180
	lo1 = lon1 * math.Pi / 180
	la2 = lat2 * math.Pi / 180
	lo2 = lon2 * math.Pi / 180

	r = 6378100 // Earth radius in METERS

	// calculate
	h := hsin(la2-la1) + math.Cos(la1)*math.Cos(la2)*hsin(lo2-lo1)

	return 2 * r * math.Asin(math.Sqrt(h))
}

// haversin(θ) function
func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}
