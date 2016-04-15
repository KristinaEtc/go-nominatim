package main

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	//"fmt"
	_ "github.com/lib/pq"
	"log"
	"os"
	//"strconv"
	// "testing"
	//"fmt"
)

var configFile = "config.json"

var testFile = "test.txt"
var numOfReq = 100

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
	speedTest      bool
}

func (l *LocationParams) getParams() {

	flag.StringVar(&(l.format), "f", "json", "format")
	flag.Float64Var(&l.lat, "a", 53.902238, "lat")
	flag.Float64Var(&l.lon, "b", 27.561916, "log")
	flag.IntVar(&l.zoom, "z", 18, "zoom")
	flag.BoolVar(&l.addressDetails, "d", false, "addressDetails")
	flag.BoolVar(&l.speedTest, "t", false, "speed test")

	flag.Parse()
	//flag.Parsed()

	return
}

func (l *LocationParams) configurateDB() {

	file, err := os.Open(configFile)
	if err != nil {
		log.Printf("No configurate file")
	} else {
		defer file.Close()
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

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	l := LocationParams{}

	l.configurateDB()
	l.getParams()

	//log.Print(l)

	if l.speedTest == false {
		numOfReq = 1
	} else {
		f, err := os.OpenFile(testFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		//log.SetOutput(f)
	}

	sqlOpenStr := "dbname=" + l.config.DBname +
		" host=" + l.config.Host +
		" user=" + l.config.User +
		" password=" + l.config.Password

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		log.Fatal(err)
	}
	defer reverseGeocode.Close()
	for i := 0; i < numOfReq; i++ {
		log.Println("Request:", i)

		//oReverseGeocode.SetLanguagePreference()
		reverseGeocode.SetIncludeAddressDetails(l.addressDetails)
		reverseGeocode.SetZoom(l.zoom)
		reverseGeocode.SetLocation(l.lat, l.lon)
		place := reverseGeocode.Lookup()
		log.Println("result:", i, ": ", place)
	}

}
