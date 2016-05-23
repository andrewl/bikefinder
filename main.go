package main

import "net/http"
import "encoding/json"
import "fmt"
import "os"
import "time"
import "strconv"
import "io/ioutil"
import gj "github.com/kpawlik/geojson"
import (
	"github.com/azer/crud"
	_ "github.com/go-sql-driver/mysql"
)
import "sync"

var DB *crud.DB
var ingest_mutex = &sync.Mutex{}

func main() {
	var err error

	fmt.Println(os.Getenv("DATABASE_URL"))
	DB, err = crud.Connect("mysql", os.Getenv("DATABASE_URL"))

	if err != nil {
		panic(err)
	}

	err = DB.CreateTables(dockingStation{})
	if err != nil {
		fmt.Println("error: %v", err)
		return
	}

	//@todo - returns all stations. get from a file created using ingest?
	http.HandleFunc("/stations", station)
	//@todo - get info about a specific station - maybe history etc?
	http.HandleFunc("/station/station-id", station)

	http.HandleFunc("/ingest", ingest)
	http.Handle("/", http.FileServer(http.Dir("./static")))
	bind := fmt.Sprintf("%s:%s", os.Getenv("OPENSHIFT_GO_IP"), os.Getenv("OPENSHIFT_GO_PORT"))
	fmt.Printf("listening on %s...", bind)
	err = http.ListenAndServe(bind, nil)
	if err != nil {
		panic(err)
	}
}

func station(w http.ResponseWriter, r *http.Request) {

	dock_id := r.URL.Query().Get("i")
	scheme_id := r.URL.Query().Get("s")

	dockingStations := []*dockingStation{}
	fmt.Println(dock_id)
	err := DB.Read(&dockingStations, "WHERE scheme_id = \""+scheme_id+"\" and dock_id = \""+dock_id+"\" and time >= now() - INTERVAL 1 DAY order by time asc")
	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}

	type History struct {
		Time  time.Time
		Bikes int
		Docks int
	}

	history := []History{}
	for _, dockingStation := range dockingStations {
		var pointInTime History
		pointInTime.Time = dockingStation.Time
		pointInTime.Docks = dockingStation.Docks
		pointInTime.Bikes = dockingStation.Bikes
		history = append(history, pointInTime)
	}

	json, err := json.Marshal(history)

	w.Header().Set("Server", "bikefinder")
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)

}

/**
 * Outputs the latest docking stations to a json file.
 * @todo this does two things. exports the raw json for archiving to S3(?)
 * but also produces the list of stations for consumption by the app
 * and that should be treated as a cache which is invalidated by
 * ingest. To be split out from this function and called if the file
 * does not exist in response to a call from /stations?
 */
func write_current_stations_to_json() {

	dockingStations := []*dockingStation{}

	err := DB.Read(&dockingStations)

	features := []*gj.Feature{}
	for _, dockingStation := range dockingStations {
		properties := map[string]interface{}{"name": dockingStation.Name, "s": dockingStation.SchemeID, "i": dockingStation.DockId, "bikes": dockingStation.Bikes, "docks": dockingStation.Docks, "history": "/station"}
		lat, _ := strconv.ParseFloat(dockingStation.Lat, 64)
		lon, _ := strconv.ParseFloat(dockingStation.Lon, 64)
		p := gj.NewPoint(gj.Coordinate{gj.CoordType(lon), gj.CoordType(lat)})
		f := gj.NewFeature(p, properties, nil)
		features = append(features, f)
	}

	static_json, err := json.Marshal(features)
	err = ioutil.WriteFile("./static/stations.json", []byte(static_json), 0644)

	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}

	static_json, err = json.Marshal(dockingStations)
	err = ioutil.WriteFile("./static/latest.json", []byte(static_json), 0644)

	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}
}

func ingest(w http.ResponseWriter, r *http.Request) {

	//@todo - should this be shared or part of hire scheme package
	type Configuration struct {
		Id           string `json:"id"`
		Type         string
		Name         string `json:"name"`
		IngestionUri string `json:"ingestion_uri"`
		PublicUri    string `json:"public_uri"`
	}
	fmt.Println("config at ", os.Getenv("BIKEFINDER_CONFIG"))
	file, _ := os.Open(os.Getenv("BIKEFINDER_CONFIG"))
	decoder := json.NewDecoder(file)
	configuration := []Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	var bikeHireSchemes = []BikeHireScheme{}
	for _, schemeConfig := range configuration {
		newScheme, err := bikeHireSchemeFactory(schemeConfig.Id, schemeConfig.Type, schemeConfig.IngestionUri)
		if err != nil {
			fmt.Println("error:", err)
		}
		if err == nil {
			bikeHireSchemes = append(bikeHireSchemes, newScheme)
		}
	}

	// We put a lock here because we don't want multiple processes overwriting data. This ensures that the most recent data is always written into the DB.
	ingest_mutex.Lock()
	defer ingest_mutex.Unlock()
	msgc, errc := make(chan string), make(chan error)
	for _, bikeHireScheme := range bikeHireSchemes {
		go retrieveDockingStations(bikeHireScheme, msgc, errc)
	}

	for i := 0; i < len(bikeHireSchemes); i++ {
		select {
		case msg := <-msgc:
			fmt.Println(msg)
		case err := <-errc:
			fmt.Println(err)
		}
	}

	fmt.Println("Done Ingesting")

	write_current_stations_to_json()
}

//@todo run this on the bikeHireScheme struct.
//@todo this should be retrievestatus I guess?
func retrieveDockingStations(bikeHireScheme BikeHireScheme, msgc chan string, errc chan error) {
	fmt.Printf("%v\n", bikeHireScheme)
	var requestTime = time.Now()

	data, err := bikeHireScheme.GetDockingStations()
	if err != nil {
		errc <- err
		return
	}

	for _, dockingStation := range data {
		dockingStation.Time = requestTime
		dockingStation.Id = dockingStation.SchemeID + "-" + dockingStation.DockId
		err := DB.Update(dockingStation)
		if err != nil {
			err = DB.Create(dockingStation)
			if err != nil {
				errc <- err
				return
			}
		}
	}

	msgc <- "Done"
}
