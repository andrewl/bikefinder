package main

import "net/http"
import "encoding/json"
import "fmt"
import "os"
import "time"
import (
	"github.com/azer/crud"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	http.HandleFunc("/ingest", ingest)
	bind := fmt.Sprintf("%s:%s", os.Getenv("OPENSHIFT_GO_IP"), os.Getenv("OPENSHIFT_GO_PORT"))
	fmt.Printf("listening on %s...", bind)
	err := http.ListenAndServe(bind, nil)
	if err != nil {
		panic(err)
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
	file, _ := os.Open("/tmp/bikefinder.json")
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
			dockingStation.Time = time.Now()
			err := DB.Create(dockingStation)
			if err != nil {
				println("There was an error creating a docking station record")
				println(err)
				return
			}
		}
	}
}
