package main

import "fmt"
import "time"

type BikeHireSchemePublicConfig struct {
	Name      string `json:"name"`
	PublicUri string `json:"public_uri"`
	Location  string `json:"location"`
}

type BikeHireSchemeConfig struct {
	BikeHireSchemePublicConfig
	Id           string `json:"id"`
	Type         string
	IngestionUri string `json:"ingestion_uri"`
	CityId       string `json:"city_id"`
}

type DockingStationStatusCollector interface {
	GetDockingStationStatuses() ([]DockingStationStatus, error)
}

//Interface to enable the discovery of free docks
type FreeDockFinder interface {
	GetFreeDocksNear(lat float64, lon float64, min_docks int) error
}

//Interface to enable the discovery of bikes
type BikeFinder interface {
	GetBikesNear(lat float64, lon float64, min_bikes int) error
}

//Interface to enable the discovery of station inside a bounding box
type StationFinder interface {
	GetStationsNear(x0 float64, y0 float64, x1 float64, y1 float64) error
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

	logger.Log("msg", "Failed to find bike hire scheme "+config.Type)

	return nil, fmt.Errorf("cycleHireSchemeType %s not found", config.Type)
}

//Representation of the status of a docking station, including information about
//which scheme it's in, number of docks, bikes and current weather,
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

type DockingStations struct {
	DockingStationStatuses []*DockingStationStatus
}
