package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
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

// PostEvent handler
func postEvent(w rest.ResponseWriter, r *rest.Request) {

	decoder := json.NewDecoder(r.Body)
	event := Event{}
	err := decoder.Decode(&event)

	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println(event)
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

	w.WriteJson(map[string]string{"message": "All Good!"})
	w.WriteHeader(http.StatusOK)

}
