Bikefinder
----------

Bikefinder is a collector and publisher of information for the location of bikes and docks in bicycle hire schemes. It's an API-first framework that comes with
two example applications - a zoomable world map which displays details on docking stations, and a mobile-first web page that uses HTML5 geolocation to find the nearest stations with bikes and free docks.  

Bikefinder is written in go-lang and uses MySQL-compatible DB for it's storage engine.

How to run
----------

Clone this repo and run `go build`. Then just run `./bikefinder`. The bikefinder web page listing the nearest bikes and docks will be available at
http://127.0.0.1:8881
and the zoomable world map will be available at
http://127.0.0.1/map.html

The reference deployment is at http://bikefinder-4ndrewl.rhcloud.com

API
---

Bikefinder operates as both a Collector and Publisher of information on bicycle hire schemes. Both of these are API driven. Unless you've changed any of the default environment settings (see below) the API endpoint is http://127.0.0.1:8881

`/schemes`

Returns a json array of information about all of the bicycle hire schemes. The schema is defined as follows:
```
{
  //The name of the scheme
  "name": "Barcelona",

  //The url of the scheme
  "public_uri": "http://some/url",

  //A comma separated lat-long of the scheme's location
  "location": "41.390205,2.154007"
}
```

`/stations?x0=MINX&y0=MINY&x1=MAXX&y1=MAXY`

Returns an array of GeoJSON objects for all of the stations within the lat-long bounding box defined by (MINX,MINY) -> (MAXX,MAXY) @todo document response object

`/bikes-near?x=LAT&y=LON`

Returns an array of GeoJSON objects for all of the stations with 3 or more bikes nearest to the lat-long (LAT,LON)

`/freedocks-near?x=LAT&y=LON`

Returns an array of GeoJSON objects for all of the stations with 3 or more free docks nearest to the lat-long (LAT,LON)

`/map?x=LAT&y=LON`

Returns a 256x256 PNG containing a map of the location of the docking station at (LAT, LON)

`/ingest`

Updates the data held in MySQL for each of the bicycle hire schemes.


Configuration
-------------

Configuration is largely carried out by setting the following environment variables

INGEST_IP - limits the IP address which can make the /ingest API call

JCDECAUX_API_KEY - the API key for the JCDecaux service which provides information on some bicycle hire schemes (https://developer.jcdecaux.com)

DATABASE_URL - the url to connect to the MySQL compatible database. ensure parseTime=true is appended to this, eg myuser:mypass@tcp(localhost:3306)/mydbname?parseTime=true

MAPBOX_ACCESS_TOKEN - the API key for MapBox to enable create of maps using /map api (https://www.mapbox.com/api-documentation)

OPENWEATHER_KEY - the API key for OpenWeather (http://openweathermap.com/api) - weather conditions are also stored alongside number of bikes/docks

OPENSHIFT_GO_IP - the IP address to listen for requests on

OPENSHIFT_GO_PORT - the port to listen for requests on

BIKEFINDER_CONFIG - the json file containing configuration of the bicycle hire schemes. The json consists of an array of objects with the following schema:
{
  //The internal id of the scheme
  "id": "barcelona",

  //The type of the scheme (see extending)
  "type": "Bici",

  //The url containing details of the scheme's station
  "ingestion_uri": "https://www.bicing.cat/availability_map/getJsonObject",

  //The city's id in the OpenWeather database
  "city_id": "3128760",

  //The publically displayed name of the scheme
  "name": "Barcelona",

  //The url of the scheme
  "public_uri": "",

  //A comma separated lat-long of the scheme.
  "location": "41.390205,2.154007"
}


Deployment on Heroku
--------------------
@todo

Deployment on Openshift
-----------------------
@todo
Requires go-lang cartridge
Requires MySQL cartridge

Extending
------------------
Adding new schemes with an existing scheme type
- @todo - should be just adding new entry in json

Adding new scheme type
- @todo - need to create new struct which implements the DockingStationStatusCollector interface and add it to the bikeHireSchemeFactory function

