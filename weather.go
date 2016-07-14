package main

import "os"
import "net/http"
import "encoding/json"

type WeatherConditions struct {
	Precipitation int
	Temperature   int
	Windspeed     int
}

type WeatherConditionsCollector interface {
	GetCurrentWeatherConditions() (WeatherConditions, error)
}

type OpenWeather struct {
}

func (ow OpenWeather) GetCurrentWeatherConditions(cityId string) (WeatherConditions, error) {

	if cityId == "" {
		return WeatherConditions{}, nil
	}

	url := "http://api.openweathermap.org/data/2.5/weather?APPID=" + os.Getenv("OPENWEATHER_KEY") + "&id=" + cityId
	logger.Log("msg", "Retrieving weather from", "url", url)
	resp, err := http.Get(url)
	if err != nil {
		logger.Log("err", err)
		return WeatherConditions{}, err
	}

	defer resp.Body.Close()

	type OpenWeatherData struct {
		Name string `json:"name"`
		Main struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
		Precipitation float64
		Wind          struct {
			Speed float64 `json:"speed"`
		} `json:"wind"`
	}

	var d OpenWeatherData

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		logger.Log("msg", "There was an error decoding weather data", "err", err)
		return WeatherConditions{}, err
	}

	var c WeatherConditions
	c.Precipitation = int(d.Precipitation)
	c.Temperature = int(d.Main.Temp / 10)
	c.Windspeed = int(d.Wind.Speed)

	return c, nil
}
