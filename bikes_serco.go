package main

import "net/http"
import "encoding/xml"

type SercoScheme struct {
	//@todo - should this all be config?
	name string
	url  string
}

func (scheme SercoScheme) GetDockingStations() (dockingStations []dockingStation, err error) {

	type sercoDockingStation struct {
		Id    string `xml:"id"`
		Name  string `xml:"name"`
		Lat   string `xml:"lat"`
		Lon   string `xml:"long"`
		Bikes int    `xml:"nbBikes"`
		Docks int    `xml:"nbEmptyDocks"`
	}

	type sercoDockingStations struct {
		DockingStations []sercoDockingStation `xml:"station"`
	}

	resp, err := http.Get(scheme.url)
	if err != nil {
		return dockingStations, err
	}

	defer resp.Body.Close()

	var d sercoDockingStations

	if err := xml.NewDecoder(resp.Body).Decode(&d); err != nil {
		return dockingStations, err
	}

	for _, sercoDockingStation := range d.DockingStations {
		var ds dockingStation
		ds.Lat = sercoDockingStation.Lat
		ds.Lon = sercoDockingStation.Lon
		ds.DockId = sercoDockingStation.Id
		ds.Name = sercoDockingStation.Name
		ds.Bikes = sercoDockingStation.Bikes
		ds.Docks = sercoDockingStation.Docks
		ds.SchemeID = scheme.name
		dockingStations = append(dockingStations, ds)
	}

	return dockingStations, err
}
