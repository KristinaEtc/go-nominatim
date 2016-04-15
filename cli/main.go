package main

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	//"fmt"
	"bufio"
	_ "github.com/lib/pq"
	"log"
	"os"
	"strconv"
	// "testing"
	//"fmt"
	"strings"
)

var configFile = "config.json"

var testFile = "test.csv"
var numOfReq = 0

type ConfigDB struct {
	DBname, Host, User, Password string
}

type LocationParams struct {
	lat  float64
	lon  float64
	zoom int
}

type Params struct {
	locParams      LocationParams
	format         string
	addressDetails bool
	sqlOpenStr     string
	config         ConfigDB
	db             *sql.DB
	speedTest      bool
}

func (p *Params) getParams() {

	flag.StringVar(&(p.format), "f", "json", "format")
	flag.Float64Var(&p.locParams.lat, "a", 53.902238, "lat")
	flag.Float64Var(&p.locParams.lon, "b", 27.561916, "log")
	flag.IntVar(&p.locParams.zoom, "z", 18, "zoom")
	flag.BoolVar(&p.addressDetails, "d", false, "addressDetails")
	flag.BoolVar(&p.speedTest, "t", false, "speed test")
	flag.StringVar(&(testFile), "n", testFile, "name of your test file")

	flag.Parse()
	//flag.Parsed()

	return
}

func (p *Params) configurateDB() {

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
		p.config = configuration
	}
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	p := Params{}

	p.configurateDB()
	p.getParams()

	//log.Print(p)
	var l []LocationParams

	if p.speedTest == false {
		numOfReq = 1
		l = append(l, p.locParams)
	} else {
		file, err := os.Open(testFile)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		defer file.Close()

		reader := bufio.NewReader(file)
		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			locs := scanner.Text()

			locSlice := strings.Split(locs, ",")
			locStr := LocationParams{}
			locStr.lat, err = strconv.ParseFloat(locSlice[0], 32)
			if err != nil {
				log.Print(err)
				continue
			}
			//log.Println(locStr.lat)
			locStr.lon, err = strconv.ParseFloat(locSlice[1], 32)
			if err != nil {
				log.Print(err)
				continue
			}
			//log.Println(locStr.lon)
			locStr.zoom, err = strconv.Atoi(locSlice[2])
			if err != nil {
				log.Print(err)
				continue
			}
			//log.Println(locStr.zoom)
			l = append(l, locStr)
		}
		for _, data := range l {
			log.Printf("%f %f %d\n", data.lat, data.lon, data.zoom)
			numOfReq++
		}
	}
	sqlOpenStr := "dbname=" + p.config.DBname +
		" host=" + p.config.Host +
		" user=" + p.config.User +
		" password=" + p.config.Password

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		log.Fatal(err)
	}
	defer reverseGeocode.Close()
	for _, data := range l {
		//log.Println("Request:", i)

		//oReverseGeocode.SetLanguagePreference()
		reverseGeocode.SetIncludeAddressDetails(p.addressDetails)
		reverseGeocode.SetZoom(data.zoom)
		reverseGeocode.SetLocation(data.lat, data.lon)
		place := reverseGeocode.Lookup()
		log.Println(place)
	}

}
