package main

import "net/http"
import "encoding/json"
import "os"
import "strconv"
import "fmt"

type JCDecauxScheme struct {
	OpenWeather
	name   string
	url    string
	cityId string
}

func (scheme JCDecauxScheme) GetDockingStationStatuses() ([]DockingStationStatus, error) {

	weather, _ := scheme.GetCurrentWeatherConditions(scheme.cityId)

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
	logger.Log("msg", "Retrieving JCDecauxScheme statuses", "url", url)
	resp, err := http.Get(url)
	if err != nil {
		logger.Log("msg", "There was an error retrieving the statuses", "err", err)
		return []DockingStationStatus{}, err
	}

	defer resp.Body.Close()

	dockingStationStatuses := []DockingStationStatus{}

	var d = []jcDecauxStation{}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return []DockingStationStatus{}, err
	}

	for _, station := range d {
		var ds DockingStationStatus
		//@todo add lat-long etc
		ds.SchemeID = scheme.name
		ds.DockId = strconv.Itoa(station.Number)
		ds.Name = station.Name
		ds.Lat = strconv.FormatFloat(station.Position.Lat, 'f', -1, 64)
		ds.Lon = strconv.FormatFloat(station.Position.Lng, 'f', -1, 64)
		ds.Bikes = int(station.Bikes)
		ds.Docks = int(station.Docks)
		ds.Precipitation = weather.Precipitation
		ds.Temperature = weather.Temperature
		ds.Windspeed = weather.Windspeed
		dockingStationStatuses = append(dockingStationStatuses, ds)
	}

	return dockingStationStatuses, nil
}
