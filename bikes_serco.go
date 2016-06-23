package main

import "net/http"
import "encoding/xml"

type SercoScheme struct {
	OpenWeather
	//@todo - should this all be config?
	name   string
	url    string
	cityId string
}

func (scheme SercoScheme) GetDockingStationStatuses() (dockingStationStatuses []DockingStationStatus, err error) {

	weather, _ := scheme.GetCurrentWeatherConditions(scheme.cityId)

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
		return dockingStationStatuses, err
	}

	defer resp.Body.Close()

	var d sercoDockingStations

	if err := xml.NewDecoder(resp.Body).Decode(&d); err != nil {
		return dockingStationStatuses, err
	}

	for _, sercoDockingStation := range d.DockingStations {
		var ds DockingStationStatus
		ds.Lat = sercoDockingStation.Lat
		ds.Lon = sercoDockingStation.Lon
		ds.DockId = sercoDockingStation.Id
		ds.Name = sercoDockingStation.Name
		ds.Bikes = sercoDockingStation.Bikes
		ds.Docks = sercoDockingStation.Docks
		ds.SchemeID = scheme.name
		ds.Precipitation = weather.Precipitation
		ds.Temperature = weather.Temperature
		ds.Windspeed = weather.Windspeed
		dockingStationStatuses = append(dockingStationStatuses, ds)
	}

	return dockingStationStatuses, err
}
