package main

import "fmt"
import "time"

type BikeHireScheme interface {
	GetDockingStations() ([]dockingStation, error)
}

func bikeHireSchemeFactory(cycleHireSchemeType string, url string) (BikeHireScheme, error) {

	switch cycleHireSchemeType {
	case "Villo":
		//		return VilloScheme{url: url, name: cycleHireSchemeType}, nil
	case "Serco":
		return SercoScheme{url: url, name: cycleHireSchemeType}, nil
	case "Bici":
		return BiciScheme{url: url, name: cycleHireSchemeType}, nil
	case "JCDecaux":
		return JCDecauxScheme{url: url, name: cycleHireSchemeType}, nil

	}

	return nil, fmt.Errorf("cycleHireSchemeType %s not found", cycleHireSchemeType)
}

type dockingStation struct {
	Time     time.Time `sql:"timestamp"`
	SchemeID string
	DockId   string
	Name     string
	Lat      string
	Lon      string
	Bikes    int
	Docks    int
	// temperature * 10 degrees
	Temperature int
	// precipitation * 10 mm
	Precipitation int
}
