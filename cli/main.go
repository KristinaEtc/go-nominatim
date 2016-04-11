package main

import (
	"Nominatim/lib"
	"database/sql"
	"flag"
	_ "github.com/lib/pq"
	"log"
	//"strconv"
)

//var baseURL string = "http://192.168.240.105/nominatim/"

//"-c nominatim/reverse?format=xml&lat=53.913658&lon=27.600286&zoom=18&addressdetails=1"

func getParams() (format string, lat, lon float64, zoom int, addressDetails bool, sqlOpenStr string) {

	flag.Parsed()

	flag.StringVar(&format, "f", "json", "format")
	flag.Float64Var(&lat, "a", 53.902238, "lat")
	flag.Float64Var(&lon, "b", 27.561916, "log")
	flag.IntVar(&zoom, "z", 18, "zoom")
	flag.BoolVar(&addressDetails, "d", false, "addressDetails")
	flag.StringVar(&sqlOpenStr, "s", "dbname=nominatim", "sqlOpenStr")

	flag.Parse()

	//fmt.Println(format, lat, lon, zoom, addressDetails)

	return
}

func getDB(sqlOpenStr string) sql.DB {
	db, err := sql.Open("postgres", sqlOpenStr)
	if err != nil {
		log.Fatal(err) ///qqqqqqq

	}
	return *db
}

func main() {

	format, lat, lon, zoom, addressDetails, sqlOpenStr := getParams()
	log.Print(format, lat, lon, zoom, addressDetails, sqlOpenStr)

	db := getDB(sqlOpenStr)

	oReverseGeocode := Nominatim.NewReverseGeocode(db)
	//oReverseGeocode.SetLanguagePreference()
	oReverseGeocode.SetIncludeAddeessDetails(addressDetails)
	oReverseGeocode.SetZoom(zoom)
	oReverseGeocode.SetLocation(lat, lon)

	oReverseGeocode.Lookup()

}
