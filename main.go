package main

import "net/http"
import "encoding/json"
import "fmt"
import "os"
import "time"
import "strconv"
import "io/ioutil"
import gj "github.com/kpawlik/geojson"
import (
	"github.com/azer/crud"
	_ "github.com/go-sql-driver/mysql"
)
import "sync"
import "github.com/aws/aws-sdk-go/aws"
import "github.com/aws/aws-sdk-go/aws/session"
import "github.com/aws/aws-sdk-go/service/s3"
import "bytes"

var DB *crud.DB
var ingest_mutex = &sync.Mutex{}

func main() {
	var err error

	fmt.Println(os.Getenv("DATABASE_URL"))
	DB, err = crud.Connect("mysql", os.Getenv("DATABASE_URL"))

	if err != nil {
		panic(err)
	}

	err = DB.CreateTables(DockingStationStatus{})
	if err != nil {
		fmt.Println("error: %v", err)
		return
	}

	http.HandleFunc("/bikes-near", getBikesNear)
	http.HandleFunc("/freedocks-near", getFreeDocksNear)
	http.HandleFunc("/ingest", ingest)
	http.Handle("/", http.FileServer(http.Dir("./static")))
	bind := fmt.Sprintf("%s:%s", os.Getenv("OPENSHIFT_GO_IP"), os.Getenv("OPENSHIFT_GO_PORT"))
	fmt.Printf("listening on %s...", bind)
	err = http.ListenAndServe(bind, nil)
	if err != nil {
		panic(err)
	}
}

func getFreeDocksNear(w http.ResponseWriter, r *http.Request) {
	writeDockingStations(w, r, "bikes")
}

func getBikesNear(w http.ResponseWriter, r *http.Request) {
	writeDockingStations(w, r, "docks")
}

func writeDockingStations(w http.ResponseWriter, r *http.Request, filter_type string) {

	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, _ := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)

	var dockingStations DockingStations

	if filter_type == "bikes" {
		dockingStations.GetBikesNear(lat, lon, 3)
	} else {
		dockingStations.GetFreeDocksNear(lat, lon, 3)
	}

	//@todo deal with 500s

	/**
	if err != nil {
		fmt.Println("error")
		return
	}
	*/

	features := []*gj.Feature{}
	for _, dockingStationStatus := range dockingStations.DockingStationStatuses {
		properties := map[string]interface{}{"name": dockingStationStatus.Name, "i": dockingStationStatus.DockId, "bikes": dockingStationStatus.Bikes, "docks": dockingStationStatus.Docks, "updated": dockingStationStatus.Time}
		lat, _ := strconv.ParseFloat(dockingStationStatus.Lat, 64)
		lon, _ := strconv.ParseFloat(dockingStationStatus.Lon, 64)
		p := gj.NewPoint(gj.Coordinate{gj.CoordType(lon), gj.CoordType(lat)})
		f := gj.NewFeature(p, properties, nil)
		features = append(features, f)
	}

	ret, err := json.Marshal(features)

	if err != nil {
		fmt.Println("error")
	}

	w.Header().Set("Server", "bikefinder")
	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)

}

func (ds *DockingStations) GetBikesNear(lat float64, lon float64, min_bikes int) {
	sql := fmt.Sprintf("WHERE bikes >= %d ORDER BY (POW((lon-%.4f),2) + POW((lat-%.4f),2)) LIMIT 10", min_bikes, lon, lat)
	err := DB.Read(&ds.DockingStationStatuses, sql)

	if err != nil {
		fmt.Println("error")
		fmt.Println(err)
	}
}

func (ds *DockingStations) GetFreeDocksNear(lat float64, lon float64, min_docks int) {
	sql := fmt.Sprintf("WHERE docks >= %d ORDER BY (POW((lon-%.4f),2) + POW((lat-%.4f),2)) LIMIT 10", min_docks, lon, lat)
	err := DB.Read(&ds.DockingStationStatuses, sql)

	if err != nil {
		fmt.Println("error")
		fmt.Println(err)
	}
}

/**
 * Outputs the latest docking stations to a json file.
 * @todo this does two things. exports the raw json for archiving to S3(?)
 * but also produces the list of stations for consumption by the app
 * and that should be treated as a cache which is invalidated by
 * ingest. To be split out from this function and called if the file
 * does not exist in response to a call from /stations?
 */
func write_current_stations_to_json() {

	dockingStationStatuses := []*DockingStationStatus{}

	err := DB.Read(&dockingStationStatuses)

	features := []*gj.Feature{}
	for _, dockingStation := range dockingStationStatuses {
		properties := map[string]interface{}{"name": dockingStation.Name, "s": dockingStation.SchemeID, "i": dockingStation.DockId, "bikes": dockingStation.Bikes, "docks": dockingStation.Docks, "history": "/station"}
		lat, _ := strconv.ParseFloat(dockingStation.Lat, 64)
		lon, _ := strconv.ParseFloat(dockingStation.Lon, 64)
		p := gj.NewPoint(gj.Coordinate{gj.CoordType(lon), gj.CoordType(lat)})
		f := gj.NewFeature(p, properties, nil)
		features = append(features, f)
	}

	static_json, err := json.Marshal(features)
	err = ioutil.WriteFile("./static/stations.json", []byte(static_json), 0644)

	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}

	static_json, err = json.Marshal(dockingStationStatuses)
	err = ioutil.WriteFile("./static/latest.json", []byte(static_json), 0644)

	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}

	svc := s3.New(session.New(), &aws.Config{Region: aws.String("us-west-2")})
	params := &s3.PutObjectInput{
		Bucket: aws.String("al-bikefinder"),                             // Required
		Key:    aws.String(time.Now().Format("2006/01/02/150405.json")), // Required
		Body:   bytes.NewReader([]byte(static_json))}

	resp, err := svc.PutObject(params)

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		fmt.Println(err.Error())
		return
	}

	// Pretty-print the response data.
	fmt.Println(resp)

}

func ingest(w http.ResponseWriter, r *http.Request) {

	fmt.Println("config at ", os.Getenv("BIKEFINDER_CONFIG"))
	file, _ := os.Open(os.Getenv("BIKEFINDER_CONFIG"))
	decoder := json.NewDecoder(file)
	configuration := []BikeHireSchemeConfig{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	var bikeHireSchemes = []DockingStationStatusCollector{}
	for _, schemeConfig := range configuration {
		newScheme, err := bikeHireSchemeFactory(schemeConfig)
		if err != nil {
			fmt.Println("error:", err)
		}
		if err == nil {
			bikeHireSchemes = append(bikeHireSchemes, newScheme)
		}
	}

	// We put a lock here because we don't want multiple processes overwriting data. This ensures that the most recent data is always written into the DB.
	ingest_mutex.Lock()
	defer ingest_mutex.Unlock()
	_, err = DB.Query("delete from docking_station_status")
	if err != nil {
		fmt.Println("Failed to delete from docking_station_status")
		return
	}
	msgc, errc := make(chan string), make(chan error)
	for _, bikeHireScheme := range bikeHireSchemes {
		go writeDockingStationStatusesToDB(bikeHireScheme, msgc, errc)
	}

	for i := 0; i < len(bikeHireSchemes); i++ {
		select {
		case msg := <-msgc:
			fmt.Println(msg)
		case err := <-errc:
			fmt.Println(err)
		}
	}

	fmt.Println("Done Ingesting")

	write_current_stations_to_json()
}

func writeDockingStationStatusesToDB(bikeHireScheme DockingStationStatusCollector, msgc chan string, errc chan error) {
	fmt.Printf("%v\n", bikeHireScheme)

	var requestTime = time.Now()

	data, err := bikeHireScheme.GetDockingStationStatuses()
	if err != nil {
		errc <- err
		return
	}

	for _, ds := range data {
		ds.RequestTime = requestTime
		ds.SchemeDockId = ds.SchemeID + "-" + ds.DockId
		err = DB.Create(ds)
		if err != nil {
			errc <- err
			return
		}
	}

	msgc <- "Done"
}
