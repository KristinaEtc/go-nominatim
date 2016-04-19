package main

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	"github.com/bitly/go-nsq"
	_ "github.com/lib/pq"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

var configFile = "../cli/config.json"

var numOfReq = 0

type ConfigDB struct {
	DBname, Host, User, Password string
}

const (
	HOST               string = "localhost"
	PORT               string = ":4150"
	topicToSubscribe   string = "topicName"
	channelToSubscribe string = "channelName"
)

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

func (p *Params) getParams() {

	//testing arguments for valid
	flag.Parsed()

	flag.StringVar(&(p.format), "f", "json", "format")
	flag.IntVar(&p.locParams.zoom, "z", 18, "zoom")
	flag.BoolVar(&p.addressDetails, "d", false, "addressDetails")

	flag.Parse()

	return
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	params := Params{}
	params.configurateDB()
	params.getParams()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	config := nsq.NewConfig()
	consumerPointer, err := nsq.NewConsumer(topicToSubscribe, channelToSubscribe, config)
	if err != nil {
		log.Panic(err)
		return
	}
	consumerPointer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		rawMsg := string(message.Body[:len(message.Body)])
		log.Printf("Got a message: %s", rawMsg)

		params.addCoordinatesToStruct(rawMsg)
		place := params.getLocationFromNominatim()

		if place != nil {
			placeJSON := getLocationJSON(*place)
			if placeJSON != "" {
				log.Println(placeJSON)
			}
		}
		//wg.Done()
		return nil
	}))

	err = consumerPointer.ConnectToNSQD(HOST + PORT)
	if err != nil {
		log.Panic("Could not connect")
	}
	wg.Wait()

}

func (p *Params) addCoordinatesToStruct(rawMsg string) {

	var err error
	coordinates := strings.Split(rawMsg, ",")
	p.locParams.lat, err = strconv.ParseFloat(coordinates[0], 32)
	if err != nil {
		log.Print(err)
		return
	}
	p.locParams.lon, err = strconv.ParseFloat(coordinates[1], 32)
	if err != nil {
		log.Print(err)
		return
	}

	p.locParams.zoom, err = strconv.Atoi(coordinates[2])
	if err != nil {
		log.Print(err)
		return
	}
	//log.Printf("-%f-%f-%d-", p.locParams.lat, p.locParams.lon, p.locParams.zoom)
}

func (p *Params) getLocationFromNominatim() *Nominatim.DataWithoutDetails {

	sqlOpenStr := "dbname=" + p.config.DBname +
		" host=" + p.config.Host +
		" user=" + p.config.User +
		" password=" + p.config.Password

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		log.Fatal(err)
	}
	defer reverseGeocode.Close()

	//oReverseGeocode.SetLanguagePreference()
	reverseGeocode.SetIncludeAddressDetails(p.addressDetails)
	reverseGeocode.SetZoom(p.locParams.zoom)
	reverseGeocode.SetLocation(p.locParams.lat, p.locParams.lon)
	place, err := reverseGeocode.Lookup()
	if err != nil {
		return nil
	}
	//log.Printf("%v", *place)
	return place
}

func getLocationJSON(data Nominatim.DataWithoutDetails) string {

	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return ""
	}
	//log.Println("\n\nsdfnsdfsdfsdf\n\n")
	//os.Stdout.Write(dataJSON)
	//log.Println(dataJSON)
	return string(dataJSON)
}
