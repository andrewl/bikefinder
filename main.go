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

func main() {
	http.HandleFunc("/station", station)
	http.HandleFunc("/ingest", ingest)
	http.Handle("/", http.FileServer(http.Dir("./static")))
	bind := fmt.Sprintf("%s:%s", os.Getenv("OPENSHIFT_GO_IP"), os.Getenv("OPENSHIFT_GO_PORT"))
	fmt.Printf("listening on %s...", bind)
	err := http.ListenAndServe(bind, nil)
	if err != nil {
		panic(err)
	}
}

func station(w http.ResponseWriter, r *http.Request) {

	dock_id := r.URL.Query().Get("i")
	scheme_id := r.URL.Query().Get("s")

	//@todo 'globalise' this
	var DB *crud.DB
	fmt.Println(os.Getenv("DATABASE_URL"))
	DB, _ = crud.Connect("mysql", os.Getenv("DATABASE_URL"))

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

//func stations(w http.ResponseWriter, r *http.Request) {
func write_stations() {

	//@todo 'globalise' this
	var DB *crud.DB
	fmt.Println(os.Getenv("DATABASE_URL"))
	DB, _ = crud.Connect("mysql", os.Getenv("DATABASE_URL"))

	dockingStations := []*dockingStation{}
	err := DB.Read(&dockingStations, "WHERE time = (SELECT max(time) from docking_station)")
	//err = DB.Read(&dockingStations)
	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}

	features := []*gj.Feature{}
	for _, dockingStation := range dockingStations {
		properties := map[string]interface{}{"name": dockingStation.Name, "s": dockingStation.SchemeID, "i": dockingStation.DockId, "bikes": dockingStation.Bikes, "docks": dockingStation.Docks, "history": "/station"}
		lat, _ := strconv.ParseFloat(dockingStation.Lat, 64)
		lon, _ := strconv.ParseFloat(dockingStation.Lon, 64)
		p := gj.NewPoint(gj.Coordinate{gj.CoordType(lon), gj.CoordType(lat)})
		f := gj.NewFeature(p, properties, nil)
		features = append(features, f)
	}

	json, err := json.Marshal(features)

	err = ioutil.WriteFile("./static/stations.json", []byte(json), 0644)

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
	fmt.Printf("%v\n", configuration)

	var bikeHireSchemes = []BikeHireScheme{}
	for _, schemeConfig := range configuration {
		newScheme, err := bikeHireSchemeFactory(schemeConfig.Type, schemeConfig.IngestionUri)
		if err != nil {
			fmt.Println("error:", err)
		}
		if err == nil {
			bikeHireSchemes = append(bikeHireSchemes, newScheme)
		}
	}

	var DB *crud.DB
	// fmt.Println(os.Getenv("DATABASE_URL"))
	DB, err = crud.Connect("mysql", os.Getenv("DATABASE_URL"))
	err = DB.Ping()

	err = DB.CreateTables(dockingStation{})
	if err != nil {
		fmt.Println("error: %v", err)
		return
	}

	//@todo - delete data older than one day. @todo - how to archive this?
	//one option is to backup to s3 then use lambda to run some task to
	//add data to a larger db

	//@todo - these should be run concurrently!!!
	msgc, errc := make(chan string), make(chan error)
	for _, bikeHireScheme := range bikeHireSchemes {
		go retrieveDockingStations(bikeHireScheme, DB, msgc, errc)
	}

	for i := 0; i < len(bikeHireSchemes); i++ {
		select {
		case msg := <-msgc:
			fmt.Println(msg)
		case err := <-errc:
			fmt.Println(err)
		}
	}

	fmt.Println("done")

	write_stations()
}

func retrieveDockingStations(bikeHireScheme BikeHireScheme, DB *crud.DB, msgc chan string, errc chan error) {
	fmt.Printf("%v\n", bikeHireScheme)
	var requestTime = time.Now()

	data, err := bikeHireScheme.GetDockingStations()
	if err != nil {
		errc <- err
		return
	}

	for _, dockingStation := range data {
		//println(dockingStation.Name)
		dockingStation.Time = requestTime
		err := DB.Create(dockingStation)
		if err != nil {
			errc <- err
			return
		}
	}

	msgc <- "Done"
}
