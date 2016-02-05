package main

import "net/http"
import "encoding/json"
import "fmt"
import "os"
import "time"
import "strconv"
import gj "github.com/kpawlik/geojson"
import (
	"github.com/azer/crud"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	http.HandleFunc("/stations", stations)
	http.HandleFunc("/ingest", ingest)
	http.Handle("/", http.FileServer(http.Dir("./static")))
	bind := fmt.Sprintf("%s:%s", os.Getenv("OPENSHIFT_GO_IP"), os.Getenv("OPENSHIFT_GO_PORT"))
	fmt.Printf("listening on %s...", bind)
	err := http.ListenAndServe(bind, nil)
	if err != nil {
		panic(err)
	}
}

func stations(w http.ResponseWriter, r *http.Request) {
	//@todo 'globalise' this
	var DB *crud.DB
	fmt.Println(os.Getenv("DATABASE_URL"))
	DB, _ = crud.Connect("mysql", os.Getenv("DATABASE_URL"))

	/**
	maxtime := time.Time{}
	err := DB.Read(maxtime, "select max(time) from docking_station")
	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}
	fmt.Printf("%+v", maxtime)
	*/

	dockingStations := []*dockingStation{}
	err := DB.Read(&dockingStations, "WHERE time = (SELECT max(time) from docking_station)")
	//err = DB.Read(&dockingStations)
	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}

	features := []*gj.Feature{}
	for _, dockingStation := range dockingStations {
		properties := map[string]interface{}{"name": dockingStation.Name, "code": dockingStation.DockId, "bikes": dockingStation.Bikes, "docks": dockingStation.Docks}
		lat, _ := strconv.ParseFloat(dockingStation.Lat, 64)
		lon, _ := strconv.ParseFloat(dockingStation.Lon, 64)
		p := gj.NewPoint(gj.Coordinate{gj.CoordType(lon), gj.CoordType(lat)})
		f := gj.NewFeature(p, properties, nil)
		features = append(features, f)
	}

	json, err := json.Marshal(features)

	w.Header().Set("Server", "bikefinder")
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)

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

	var requestTime = time.Now()

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
	fmt.Println(os.Getenv("DATABASE_URL"))
	DB, err = crud.Connect("mysql", os.Getenv("DATABASE_URL"))
	err = DB.Ping()

	err = DB.CreateTables(dockingStation{})
	if err != nil {
		fmt.Println("error: %v", err)
		return
	}

	//@todo - these should be run concurrently!!!
	for _, bikeHireScheme := range bikeHireSchemes {
		fmt.Printf("%v\n", bikeHireScheme)

		data, err := bikeHireScheme.GetDockingStations()
		if err != nil {
			println(err)
			return
		}

		for _, dockingStation := range data {
			println(dockingStation.Name)
			dockingStation.Time = requestTime
			err := DB.Create(dockingStation)
			if err != nil {
				println("There was an error creating a docking station record")
				println(err)
				return
			}
		}
	}
}
