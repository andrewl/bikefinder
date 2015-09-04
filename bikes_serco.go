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
		Bikes int64  `xml:"nbBikes"`
		Docks int64  `xml:"nbEmptyDocks"`
	}

	type sercoDockingStations struct {
		DockingStations []dockingStation `xml:"station"`
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

	for _, dockingStation := range d.DockingStations {
		dockingStation.SchemeID = scheme.name
		dockingStations = append(dockingStations, dockingStation)
	}

	return dockingStations, err
}
