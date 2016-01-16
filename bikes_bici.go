package main

import "net/http"
import "encoding/json"
import "strconv"
import "log"

type BiciScheme struct {
	name string
	url  string
}

func (scheme BiciScheme) GetDockingStations() ([]dockingStation, error) {

	type biciStations struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Lat   string `json:"lat"`
		Lon   string `json:"lon"`
		Bikes string `json:"bikes"`
		Docks string `json:"slots"`
	}

	resp, err := http.Get(scheme.url)
	if err != nil {
		log.Print("There was an error retrieving ", scheme.url)
		return []dockingStation{}, err
	}

	defer resp.Body.Close()

	dockingStations := []dockingStation{}

	var bs = []biciStations{}

	if err := json.NewDecoder(resp.Body).Decode(&bs); err != nil {
		log.Print("Failed to decode json")
		return []dockingStation{}, err
	}

	for _, station := range bs {
		var ds dockingStation
		//@todo add lat-long etc
		ds.SchemeID = scheme.name
		ds.DockId = station.Id
		ds.Lat = station.Lat
		ds.Lon = station.Lon
		ds.Name = station.Name
		ds.Bikes, err = strconv.Atoi(station.Bikes)
		ds.Docks, err = strconv.Atoi(station.Docks)
		dockingStations = append(dockingStations, ds)
	}

	return dockingStations, nil
}
