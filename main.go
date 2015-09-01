package main

import "net/http"
import "encoding/xml"
import "encoding/json"
import "fmt"
import "os"
import "net/url"
import "log"
import "strconv"
import "github.com/influxdb/influxdb/client"
import "github.com/pierrre/geohash"
import "strings"

//@todo get these from config?
const (
	MyHost = "localhost"
	MyPort = 8086
	MyDB   = "bikemap"
	//@todo - tidy up nomenclature
	MyMeasurement = "locations"
)

type BikeHireScheme interface {
	GetDockingStations() ([]dockingStation, error)
}

type BiciScheme struct {
	name string
	url  string
}

type VilloScheme struct {
	name string
	url  string
}

type SercoScheme struct {
	name string
	url  string
}

func cycleHireSchemeFactory(cycleHireSchemeType string, url string) (BikeHireScheme, error) {

	switch cycleHireSchemeType {
	case "Villo":
		return VilloScheme{url: url}, nil
	case "Serco":
		return SercoScheme{url: url}, nil
	case "Bici":
		return BiciScheme{url: url}, nil

	}

	return nil, fmt.Errorf("cycleHireSchemeType not found")
}

//@todo - add the scheme id in here?
type dockingStation struct {
	SchemeID string
	Id       string `xml:"id"`
	Name     string `xml:"name"`
	Lat      string `xml:"lat"`
	Lon      string `xml:"long"`
	Bikes    int64  `xml:"nbBikes"`
	Docks    int64  `xml:"nbEmptyDocks"`
}

func (scheme BiciScheme) GetDockingStations() ([]dockingStation, error) {

	type biciStations struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Lat   string `json:"lat"`
		Lon   string `json:"lon"`
		Bikes string `json:"bikes"`
		Docks string `json:"slots"`
	}

	resp, err := http.Get(scheme.url)
	if err != nil {
		return []dockingStation{}, err
	}

	defer resp.Body.Close()

	dockingStations := []dockingStation{}

	var bs = []biciStations{}

	if err := json.NewDecoder(resp.Body).Decode(&bs); err != nil {
		return []dockingStation{}, err
	}

	for _, station := range bs {
		var ds dockingStation
		//@todo add lat-long etc
		ds.SchemeID = "bici"
		ds.Id = station.Id
		ds.Lat = station.Lat
		ds.Lon = station.Lon
		ds.Name = station.Name
		ds.Bikes, err = strconv.ParseInt(station.Bikes, 10, 64)
		ds.Docks, err = strconv.ParseInt(station.Docks, 10, 64)
		dockingStations = append(dockingStations, ds)
	}

	return dockingStations, nil
}

func (scheme VilloScheme) GetDockingStations() ([]dockingStation, error) {

	type carto struct {
		Markers struct {
			Marker []struct {
				Number  string `xml:"number,attr"`
				Address string `xml:"address,attr"`
			} `xml:"marker"`
		} `xml:"markers"`
	}

	type cartoStation struct {
		Available string `xml:"available"`
		Free      string `xml:"free"`
	}

	resp, err := http.Get(scheme.url)
	if err != nil {
		return []dockingStation{}, err
	}

	defer resp.Body.Close()

	var d carto

	if err := xml.NewDecoder(resp.Body).Decode(&d); err != nil {
		return []dockingStation{}, err
	}

	dockingStations := []dockingStation{}

	//@todo use go subroutines
	for _, value := range d.Markers.Marker {
		//@todo - remove hardcoded bruxelles - maybe an additional parameter?
		stationResp, err := http.Get("http://en.villo.be/service/stationdetails/bruxelles/" + value.Number)
		defer stationResp.Body.Close()
		var s cartoStation
		if err == nil {
			if err := xml.NewDecoder(stationResp.Body).Decode(&s); err == nil {
				var ds dockingStation
				//@todo add lat-long etc
				ds.SchemeID = "foo"
				ds.Name = value.Address
				ds.Bikes, err = strconv.ParseInt(s.Available, 10, 64)
				ds.Docks, err = strconv.ParseInt(s.Free, 10, 64)
				dockingStations = append(dockingStations, ds)
			}
		}
	}

	return dockingStations, nil

}

func (scheme SercoScheme) GetDockingStations() ([]dockingStation, error) {

	type sercoDockingStations struct {
		DockingStations []dockingStation `xml:"station"`
	}

	resp, err := http.Get(scheme.url)
	if err != nil {
		return []dockingStation{}, err
	}

	defer resp.Body.Close()

	var d sercoDockingStations

	if err := xml.NewDecoder(resp.Body).Decode(&d); err != nil {
		return []dockingStation{}, err
	}

	return d.DockingStations, nil
}

func parseInfluxResults(res []client.Result) (dockingStations []dockingStation, err error) {

	for _, result := range res {
		for _, row := range result.Series {
			var ds dockingStation
			ds.Lat = row.Tags["lat"]
			ds.Lon = row.Tags["lon"]
			ds.Name = row.Tags["name"]
			ds.Id = row.Tags["location_id"]
			var bikes, _ = row.Values[0][1].(json.Number).Int64()
			var docks, _ = row.Values[0][2].(json.Number).Int64()
			ds.Bikes = int64(bikes)
			ds.Docks = int64(docks)
			dockingStations = append(dockingStations, ds)
		}
	}

	return dockingStations, err

}

func main() {
	http.HandleFunc("/ingest", ingest)
	http.HandleFunc("/get/", func(w http.ResponseWriter, r *http.Request) {
		geohash := strings.SplitN(r.URL.Path, "/", 3)[2]
		results, err := get(geohash)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data, err := parseInfluxResults(results)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(data)
	})
	http.ListenAndServe(":8163", nil)
}

func get(geohash string) (res []client.Result, err error) {
	u, err := url.Parse(fmt.Sprintf("http://%s:%d", MyHost, MyPort))
	if err != nil {
		log.Fatal(err)
	}

	conf := client.Config{
		URL:      *u,
		Username: os.Getenv("INFLUX_USER"),
		Password: os.Getenv("INFLUX_PWD"),
	}

	con, err := client.NewClient(conf)
	if err != nil {
		log.Fatal(err)
	}

	dur, ver, err := con.Ping()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	log.Printf("Happy as a Hippo! %v, %s", dur, ver)

	cmd := fmt.Sprintf("select last(docks) as docks, last(bikes) as bikes from locations where geohash =~ /^%s/ group by location_id, name, lat, lon", geohash)

	q := client.Query{
		Command:  cmd,
		Database: MyDB,
	}
	if response, err := con.Query(q); err == nil {
		if response.Error() != nil {
			return nil, response.Error()
		}
		res = response.Results
	}

	return res, nil
}

func ingest(w http.ResponseWriter, r *http.Request) {

	//@todo - should this be shared or part of hire scheme package
	type Configuration struct {
		Id   string `json:"id"`
		Type string
		Name string `json:"name"`
		Url  string `json:"url"`
	}
	file, _ := os.Open("./bikemap.json")
	decoder := json.NewDecoder(file)
	configuration := []Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%v\n", configuration)

	var bikeHireSchemes = []BikeHireScheme{}
	for _, schemeConfig := range configuration {
		newScheme, err := cycleHireSchemeFactory(schemeConfig.Type, schemeConfig.Url)
		if err != nil {
			fmt.Println("error:", err)
		}
		if err == nil {
			bikeHireSchemes = append(bikeHireSchemes, newScheme)
		}
	}

	u, err := url.Parse(fmt.Sprintf("http://%s:%d", MyHost, MyPort))
	if err != nil {
		log.Fatal(err)
	}

	conf := client.Config{
		URL:      *u,
		Username: os.Getenv("INFLUX_USER"),
		Password: os.Getenv("INFLUX_PWD"),
	}

	con, err := client.NewClient(conf)
	if err != nil {
		log.Fatal(err)
	}

	dur, ver, err := con.Ping()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Happy as a Hippo! %v, %s", dur, ver)

	//@todo - these should be run concurrently!!!
	for _, bikeHireScheme := range bikeHireSchemes {
		data, err := bikeHireScheme.GetDockingStations()
		if err != nil {
			println(err)
			return
		}

		var locations = make([]client.Point, len(data))
		var i = 0

		for _, dockingStation := range data {
			lat, _ := strconv.ParseFloat(dockingStation.Lat, 64)
			lon, _ := strconv.ParseFloat(dockingStation.Lon, 64)
			locations[i] = client.Point{
				Measurement: "locations",
				Fields: map[string]interface{}{
					//"bikes": fmt.Sprintf("%ii", dockingStation.Bikes),
					"bikes": dockingStation.Bikes,
					"docks": dockingStation.Docks,
				},
				Tags: map[string]string{
					"location_id": fmt.Sprint(dockingStation.SchemeID, "-", dockingStation.Id),
					"name":        dockingStation.Name,
					"lat":         dockingStation.Lat,
					"lon":         dockingStation.Lon,
					"geohash":     geohash.Encode(lat, lon, 12),
				},
			}
			i = i + 1
		}

		bps := client.BatchPoints{
			Points:          locations,
			Database:        MyDB,
			RetentionPolicy: "default",
		}
		_, err = con.Write(bps)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%v\n", data)
		fmt.Printf("%v\n", bps.Points)
	}

}
