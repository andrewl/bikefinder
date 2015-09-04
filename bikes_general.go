package main

import "fmt"

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

//@todo - add the scheme id in here?
type dockingStation struct {
	SchemeID string
	Id       string
	Name     string
	Lat      string
	Lon      string
	Bikes    int64
	Docks    int64
}
