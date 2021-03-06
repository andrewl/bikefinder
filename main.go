package main

import (
	"net"
	"net/http"
)
import "github.com/azer/crud"
import _ "github.com/go-sql-driver/mysql"
import "encoding/json"
import "fmt"
import "os"
import "time"
import "strconv"
import "io/ioutil"
import gj "github.com/kpawlik/geojson"
import "sync"
import "github.com/go-kit/kit/log"

var DB *crud.DB
var ingest_mutex = &sync.Mutex{}
var logger log.Logger

func main() {
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	logger.Log("msg", "Starting bikefinder")
	var err error

	db_url := os.Getenv("BF_DATABASE_URL")
	DB, err = crud.Connect("mysql", db_url)

	if err != nil {
		logger.Log("msg", "Failed to connect to db at "+db_url, "error", err)
		panic(err)
	}

	err = DB.CreateTables(DockingStationStatus{})
	if err != nil {
		logger.Log("msg", "Failed to create db table", "error", err)
		panic(err)
	}

	http.HandleFunc("/schemes", getSchemes)
	http.HandleFunc("/stations", getStationsInside)
	http.HandleFunc("/bikes-near", getBikesNear)
	http.HandleFunc("/freedocks-near", getFreeDocksNear)
	http.HandleFunc("/map", getMap)
	http.HandleFunc("/ingest", ingest)
	http.Handle("/", http.FileServer(http.Dir("./static")))
	bind := fmt.Sprintf("%s:%s", os.Getenv("BF_IP"), os.Getenv("BF_PORT"))
	logger.Log("msg", "Attempting to listen on "+bind)
	err = http.ListenAndServe(bind, nil)
	if err != nil {
		logger.Log("msg", "Failed to listen", "error", err)
		panic(err)
	}
}

/**
 * HAndler for returning a map
 */
func getMap(w http.ResponseWriter, r *http.Request) {
	//map_url := "http://foo.bar"
	lat := r.URL.Query().Get("lat")
	lon := r.URL.Query().Get("lon")
	mapbox_access_token := os.Getenv("MAPBOX_ACCESS_TOKEN")

	var map_cache_filename = "./map-cache/" + lon + "-" + lat + ".png"

	map_image, err := ioutil.ReadFile(map_cache_filename)

	if err != nil {

		var map_url = "https://api.mapbox.com/v4/mapbox.streets/url-https%3A%2F%2Fbikefinder-4ndrewl.rhcloud.com%2Fimages%2Fbike.png(" + lon + "," + lat + ")/" + lon + "," + lat + ",15/256x256.png?access_token=" + mapbox_access_token

		logger.Log("msg", "cache-miss. Retrieving map from web", "url", map_url)

		resp, err := http.Get(map_url)
		defer resp.Body.Close()
		if err != nil {
			logger.Log("msg", "Failed to retrieve map", "map_url", map_url, "err", err)
			http.Error(w, "", 500)
			return
		}

		map_image, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Log("msg", "Failed to read map body", "map_url", map_url, "err", err)
			http.Error(w, "", 500)
			return
		}

		ioutil.WriteFile(map_cache_filename, map_image, 0644)
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(map_image)
}

/**
 * Handler for returning stations inside a bounding box
 */
func getStationsInside(w http.ResponseWriter, r *http.Request) {
	writeDockingStations(w, r, "stations")
}

/**
 * Handler for returning a list of docking stations.
 */
func getFreeDocksNear(w http.ResponseWriter, r *http.Request) {
	writeDockingStations(w, r, "freedocks")
}

/**
 * Handler for returning a list of bikes.
 */
func getBikesNear(w http.ResponseWriter, r *http.Request) {
	writeDockingStations(w, r, "bikes")
}

/**
 *  or free docks, or an error if that was not possible.
 *  @see getFreeDocksNear
 *  @see getBikesNear
 */
func writeDockingStations(w http.ResponseWriter, r *http.Request, filter_type string) {

	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, _ := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	x0, _ := strconv.ParseFloat(r.URL.Query().Get("x0"), 64)
	y0, _ := strconv.ParseFloat(r.URL.Query().Get("y0"), 64)
	x1, _ := strconv.ParseFloat(r.URL.Query().Get("x1"), 64)
	y1, _ := strconv.ParseFloat(r.URL.Query().Get("y1"), 64)

	var dockingStations DockingStations
	var err error
	var ret []byte

	if filter_type == "bikes" {
		err = dockingStations.GetBikesNear(lat, lon, 4)
	}

	if filter_type == "freedocks" {
		err = dockingStations.GetBikesNear(lat, lon, 4)
	}

	if filter_type == "stations" {
		err = dockingStations.GetStationsInside(x0, y0, x1, y1)
	}

	if err == nil {
		features := []*gj.Feature{}
		for _, dockingStationStatus := range dockingStations.DockingStationStatuses {
			properties := map[string]interface{}{"name": dockingStationStatus.Name, "i": dockingStationStatus.DockId, "bikes": dockingStationStatus.Bikes, "docks": dockingStationStatus.Docks, "updated": dockingStationStatus.Time}
			lat, _ := strconv.ParseFloat(dockingStationStatus.Lat, 64)
			lon, _ := strconv.ParseFloat(dockingStationStatus.Lon, 64)
			p := gj.NewPoint(gj.Coordinate{gj.CoordType(lon), gj.CoordType(lat)})
			f := gj.NewFeature(p, properties, nil)
			features = append(features, f)
		}

		ret, err = json.Marshal(features)
	}

	if err != nil {
		//@todo set return code
		logger.Log("msg", "There was an error retrieving docking stations", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{msg: \"There was an error\"}"))
		return
	}

	w.Header().Set("Server", "bikefinder")
	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)

}

/**
 * Retrieve bikes near a lat long
 */
func (ds *DockingStations) GetBikesNear(lat float64, lon float64, min_bikes int) error {
	sql := fmt.Sprintf("WHERE bikes >= %d AND lon > %.4f and lon < %.4f and lat > %.4f and lat < %.4f ORDER BY (POW((lon-(%.4f)),2) + POW((lat-(%.4f)),2)) LIMIT 10", min_bikes, lon-0.5, lon+0.5, lat-0.5, lat+0.5, lon, lat)

	logger.Log("msg", "Getting bikes near", "sql", sql)

	err := DB.Read(&ds.DockingStationStatuses, sql)

	if err != nil {
		logger.Log("msg", "There was an error", "err", err)
	}

	return err
}

/**
 * Retrieve bikes near a lat long
 */
func (ds *DockingStations) GetStationsInside(x0 float64, y0 float64, x1 float64, y1 float64) error {
	sql := fmt.Sprintf("WHERE lon > %.4f and lon < %.4f and lat > %.4f and lat < %.4f", x0, y0, x1, y1)

	logger.Log("msg", "Getting stations inside", "sql", sql)

	err := DB.Read(&ds.DockingStationStatuses, sql)

	if err != nil {
		logger.Log("msg", "There was an error", "err", err)
	}

	return err
}

/**
 * Retrieve free docks near a lat long
 */
func (ds *DockingStations) GetFreeDocksNear(lat float64, lon float64, min_docks int) error {
	sql := fmt.Sprintf("WHERE docks >= %d ORDER BY (POW((lon-%.4f),2) + POW((lat-%.4f),2)) LIMIT 10", min_docks, lon, lat)
	err := DB.Read(&ds.DockingStationStatuses, sql)

	logger.Log("msg", "Getting docks near", "sql", sql)

	if err != nil {
		logger.Log("msg", "There was an error", "err", err)
	}

	return err
}

func getSchemes(w http.ResponseWriter, r *http.Request) {

	config_file := os.Getenv("BF_CONFIG")
	file, _ := os.Open(config_file)
	decoder := json.NewDecoder(file)
	configuration := []BikeHireSchemeConfig{}
	err := decoder.Decode(&configuration)
	if err != nil {
		logger.Log("msg", "Failed retrieve config", "err", err)
		return
	}
	var public_config = []BikeHireSchemePublicConfig{}
	for _, schemeConfig := range configuration {
		public_config = append(public_config, schemeConfig.BikeHireSchemePublicConfig)
	}

	ret, err := json.Marshal(public_config)

	w.Header().Set("Server", "bikefinder")
	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)

}

func ingest(w http.ResponseWriter, r *http.Request) {

	incoming_ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	valid_ip := os.Getenv("BF_INGEST_IP")
	if incoming_ip != valid_ip {
		logger.Log("msg", "Attempt to ingest from invalid ip address", "ip", incoming_ip)
		http.Error(w, "", 401)
		return
	}

	config_file := os.Getenv("BF_CONFIG")
	logger.Log("msg", "Startng Ingestion. Using config file at "+config_file)
	file, _ := os.Open(config_file)
	decoder := json.NewDecoder(file)
	configuration := []BikeHireSchemeConfig{}
	err := decoder.Decode(&configuration)
	if err != nil {
		logger.Log("msg", "Failed retrieve config", "err", err)
		return
	}

	var bikeHireSchemes = []DockingStationStatusCollector{}
	for _, schemeConfig := range configuration {
		newScheme, err := bikeHireSchemeFactory(schemeConfig)
		if err != nil {
			logger.Log("msg", "Failed to create bikeScheme", "err", err)
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
		logger.Log("msg", "Failed to delete old docking station status", "err", err)
		return
	}
	msgc, errc := make(chan string), make(chan error)
	for _, bikeHireScheme := range bikeHireSchemes {
		go writeDockingStationStatusesToDB(bikeHireScheme, msgc, errc)
	}

	for i := 0; i < len(bikeHireSchemes); i++ {
		select {
		case msg := <-msgc:
			logger.Log("msg", msg)
		case err := <-errc:
			logger.Log("msg", "There was an error", "err", err)
		}
	}

	logger.Log("msg", "Done Ingesting")

	logger.Log("msg", "Dumping all bikes")

	var dss []DockingStationStatus
	err = DB.Read(&dss)

	if err != nil {
		logger.Log("msg", "There was an error", "err", err)
	}
	ret, err := json.Marshal(dss)

	if err != nil {
		//@todo set return code
		logger.Log("msg", "There was an error dumping docking stations", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{msg: \"There was an error\"}"))
		return
	}

	w.Header().Set("Server", "bikefinder")
	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)

}

/**
 * Takes a bike hire scheme and writes the output to the db, informing a channel on error or completion.
 */
func writeDockingStationStatusesToDB(bikeHireScheme DockingStationStatusCollector, msgc chan string, errc chan error) {
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
