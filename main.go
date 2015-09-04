package main

import "net/http"
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
	MyDB   = "bikefinder"
	//@todo - tidy up nomenclature
	MyMeasurement = "locations"
)

func parseInfluxResults(res []client.Result) (dockingStations []dockingStation, err error) {

	for _, result := range res {
		for _, row := range result.Series {
			var ds dockingStation
			ds.Lat = row.Tags["lat"]
			ds.Lon = row.Tags["lon"]
			ds.Name = row.Tags["name"]
			ds.Id = row.Tags["location_id"]
			ds.SchemeID = row.Tags["scheme"]
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
		Id           string `json:"id"`
		Type         string
		Name         string `json:"name"`
		IngestionUri string `json:"ingestion_uri"`
		PublicUri    string `json:"public_uri"`
	}
	file, _ := os.Open("./bikefinder.json")
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
		newScheme, err := bikeHireSchemeFactory(schemeConfig.Type, schemeConfig.IngestionUri)
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
		fmt.Printf("%v\n", bikeHireScheme)

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
					"bikes": dockingStation.Bikes,
					"docks": dockingStation.Docks,
				},
				Tags: map[string]string{
					"location_id": fmt.Sprint(dockingStation.SchemeID, "-", dockingStation.Id),
					"name":        dockingStation.Name,
					"lat":         dockingStation.Lat,
					"lon":         dockingStation.Lon,
					"geohash":     geohash.Encode(lat, lon, 12),
					"scheme":      dockingStation.SchemeID,
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

	}

}
