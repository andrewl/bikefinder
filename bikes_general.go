package main

import "fmt"
import "time"

// see https://play.golang.org/p/4uhm2K5e9A !!!!
type BikeHireSchemeConfig struct {
	Id           string `json:"id"`
	Type         string
	Name         string `json:"name"`
	IngestionUri string `json:"ingestion_uri"`
	PublicUri    string `json:"public_uri"`
	CityId       string `json:"city_id"`
}

type DockingStationStatusCollector interface {
	GetDockingStationStatuses() ([]DockingStationStatus, error)
}

func bikeHireSchemeFactory(config BikeHireSchemeConfig) (DockingStationStatusCollector, error) {

	if config.Name == "" {
		config.Name = config.Id
	}

	switch config.Type {
	case "Villo":
		//		return VilloScheme{url: url, name: name}, nil
	case "Serco":
		return SercoScheme{url: config.IngestionUri, name: config.Name, cityId: config.CityId}, nil
	case "Bici":
		return BiciScheme{url: config.IngestionUri, name: config.Name, cityId: config.CityId}, nil
	case "JCDecaux":
		return JCDecauxScheme{url: config.IngestionUri, name: config.Name, cityId: config.CityId}, nil

	}

	return nil, fmt.Errorf("cycleHireSchemeType %s not found", config.Type)
}

//@todo - add json here so we can (un)marshall to/from json
type DockingStationStatus struct {
	SchemeDockId  string    `sql:"varchar(255) primary-key required"`
	RequestTime   time.Time `sql:"timestamp"`
	Time          time.Time `sql:"timestamp"`
	SchemeID      string
	DockId        string
	Name          string
	Lat           string
	Lon           string
	Bikes         int
	Docks         int
	Temperature   int
	Precipitation int
	Windspeed     int
}
