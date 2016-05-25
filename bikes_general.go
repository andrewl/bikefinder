package main

import "fmt"
import "time"

type BikeHireScheme interface {
	GetDockingStations() ([]dockingStation, error)
}

func bikeHireSchemeFactory(name string, cycleHireSchemeType string, url string) (BikeHireScheme, error) {

	if name == "" {
		name = cycleHireSchemeType
	}

	switch cycleHireSchemeType {
	case "Villo":
		//		return VilloScheme{url: url, name: name}, nil
	case "Serco":
		return SercoScheme{url: url, name: name}, nil
	case "Bici":
		return BiciScheme{url: url, name: name}, nil
	case "JCDecaux":
		return JCDecauxScheme{url: url, name: name}, nil

	}

	return nil, fmt.Errorf("cycleHireSchemeType %s not found", cycleHireSchemeType)
}

//@todo - add json here so we can (un)marshall to/from json
//@todo - this is a status more than a station
//@todo - we should probably store the time of retrieval, as well as the time of the status ffrom the service (and convert to UTC?)
type dockingStation struct {
	SchemeDockId string    `sql:"varchar(255) primary-key required"`
	Time         time.Time `sql:"timestamp"`
	SchemeID     string
	DockId       string
	Name         string
	Lat          string
	Lon          string
	Bikes        int
	Docks        int
	// temperature * 10 degrees
	Temperature int
	// precipitation * 10 mm
	Precipitation int
}
