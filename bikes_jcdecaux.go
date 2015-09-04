package main

import "net/http"
import "encoding/json"
import "os"
import "strconv"
import "log"
import "fmt"

type JCDecauxScheme struct {
	name string
	url  string
}

func (scheme JCDecauxScheme) GetDockingStations() ([]dockingStation, error) {

	type jcDecauxStation struct {
		Number   int    `json:"number"`
		Name     string `json:"name"`
		Position struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"position"`
		Bikes int64 `json:"available_bike_stands"`
		Docks int64 `json:"available_bikes"`
	}

	url := fmt.Sprint(scheme.url, "&apiKey=", os.Getenv("JCDECAUX_API_KEY"))
	resp, err := http.Get(url)
	if err != nil {
		log.Print(err.Error())
		return []dockingStation{}, err
	}

	defer resp.Body.Close()

	dockingStations := []dockingStation{}

	var d = []jcDecauxStation{}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return []dockingStation{}, err
	}

	for _, station := range d {
		var ds dockingStation
		//@todo add lat-long etc
		ds.SchemeID = scheme.name
		ds.Id = strconv.Itoa(station.Number)
		ds.Name = station.Name
		ds.Lat = strconv.FormatFloat(station.Position.Lat, 'f', -1, 64)
		ds.Lon = strconv.FormatFloat(station.Position.Lng, 'f', -1, 64)
		ds.Bikes = station.Bikes
		ds.Docks = station.Docks
		dockingStations = append(dockingStations, ds)
	}

	return dockingStations, nil
}
