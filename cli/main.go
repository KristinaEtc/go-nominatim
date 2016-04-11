package main

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	_ "github.com/lib/pq"
	"log"
	"os"
	//"strconv"
)

var configFile = "config.json"

type ConfigDB struct {
	DBname, Host, User, Password string
}

type LocationParams struct {
	format         string
	lat            float64
	lon            float64
	zoom           int
	addressDetails bool
	sqlOpenStr     string
	config         ConfigDB
	db             *sql.DB
}

//var baseURL string = "http://192.168.240.105/nominatim/"

//"-c nominatim/reverse?format=xml&lat=53.913658&lon=27.600286&zoom=18&addressdetails=1"

func (l *LocationParams) getParams() {

	flag.StringVar(&(l.format), "f", "json", "format")
	flag.Float64Var(&l.lat, "a", 53.902238, "lat")
	flag.Float64Var(&l.lon, "b", 27.561916, "log")
	flag.IntVar(&l.zoom, "z", 18, "zoom")
	flag.BoolVar(&l.addressDetails, "d", false, "addressDetails")

	flag.Parse()
	//flag.Parsed()

	return
}

func (l *LocationParams) getDB(sqlOpenStr string) {
	db, err := sql.Open("postgres", sqlOpenStr)
	if err != nil {
		log.Fatal(err) ///qqqqqqq

	}
	l.db = db
}

func (l *LocationParams) configurateDB() {

	file, err := os.Open(configFile)
	if err != nil {
		log.Printf("No configurate file")
	} else {
		decoder := json.NewDecoder(file)
		configuration := ConfigDB{}
		err := decoder.Decode(&configuration)
		if err != nil {
			log.Println("error: ", err)
		}
		l.config = configuration
	}
}

func main() {

	l := LocationParams{}

	l.configurateDB()
	l.getParams()

	//log.Print(l)

	sqlOpenStr := "dbname=" + l.config.DBname +
		" host=" + l.config.Host +
		" user=" + l.config.User +
		" password=" + l.config.Password

	l.getDB(sqlOpenStr)

	oReverseGeocode := Nominatim.NewReverseGeocode(*l.db)
	//oReverseGeocode.SetLanguagePreference()
	oReverseGeocode.SetIncludeAddeessDetails(l.addressDetails)
	oReverseGeocode.SetZoom(l.zoom)
	oReverseGeocode.SetLocation(l.lat, l.lon)

	oReverseGeocode.Lookup()

}
