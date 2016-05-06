package main

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	l4g "github.com/alecthomas/log4go"
	"github.com/bitly/go-nsq"
	_ "github.com/lib/pq"
	"os"
	"sync"
)

var configFile = "/opt/go-nsq/config.json"

var log l4g.Logger

var numOfReq = 0
var log4F = log4goFacade{&log}

type log4goFacade struct {
	log *l4g.Logger
}

func (l log4goFacade) Output(calldepth int, s string) error {
	l.log.Debug("(nsq-go) %s", s)
	return nil
}

type ConfigDB struct {
	DBname, Host, User, Password string
}

const (
	HOST               string = "localhost"
	PORT               string = ":4150"
	topicToSubscribe   string = "main"
	channelToSubscribe string = "main"
	LOGFILE            string = "/home/k/go-worker.log"
)

type Req struct {
	Lat      float64 `json: Lat`
	Lon      float64 `json:Lon`
	Zoom     int     `json:Zoom`
	ClientID string  `json:ClientID`
}

type Params struct {
	clientReq      Req
	format         string
	addressDetails bool
	sqlOpenStr     string
	config         ConfigDB
	db             *sql.DB
}

func (p *Params) configurateDB() {

	file, err := os.Open(configFile)
	if err != nil {
		log.Error("No configurate file")
	} else {

		defer file.Close()
		decoder := json.NewDecoder(file)
		configuration := ConfigDB{}

		err := decoder.Decode(&configuration)
		if err != nil {
			log.Error("error: ", err)
		}
		p.config = configuration
	}
}

func (p *Params) getParams() {

	flag.Parsed()

	flag.StringVar(&(p.format), "f", "json", "format")
	flag.IntVar(&p.clientReq.Zoom, "z", 18, "zoom")
	flag.BoolVar(&p.addressDetails, "d", false, "addressDetails")

	flag.Parse()

	return
}

func (params *Params) locationSearch() {

	wg := &sync.WaitGroup{}
	wg.Add(1)

	config := nsq.NewConfig()
	consumerPointer, err := nsq.NewConsumer(topicToSubscribe, channelToSubscribe, config)
	if err != nil {
		log.Critical(err)
		return
	}
	consumerPointer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		rawMsg := string(message.Body[:len(message.Body)])
		log.Info("Got request: %s", rawMsg)

		err := params.addCoordinatesToStruct(message.Body)
		if err != nil {
			log.Error(err)
			return err
		}
		place, err := params.getLocationFromNominatim()
		if err != nil {
			log.Error(err)
			return err
		}

		placeJSON, err := getLocationJSON(*place)
		if err != nil {
			log.Error(err)
			return err
		}

		err = params.sentData(placeJSON)
		if err != nil {
			log.Error(err)
			return err
		}

		return nil
	}))

	err = consumerPointer.ConnectToNSQD(HOST + PORT)
	if err != nil {
		log.Critical("Could not connect")
		os.Exit(1)
	}
	wg.Wait()
}

func main() {

	log = l4g.NewLogger()

	log.AddFilter("stdout", l4g.INFO, l4g.NewConsoleLogWriter())
	log.AddFilter("file", l4g.DEBUG, l4g.NewFileLogWriter(LOGFILE, true))

	params := Params{}
	params.configurateDB()
	params.getParams()
	params.locationSearch()
}

func (p *Params) sentData(msg string) error {

	config := nsq.NewConfig()
	producerPointer, err := nsq.NewProducer(HOST+PORT, config)
	if err != nil {
		log.Error("Couldn't create new worker: ", err)
		return err
	}
	log4F := log4goFacade{&log}
	producerPointer.SetLogger(log4F, nsq.LogLevelInfo)
	errPublish := producerPointer.Publish(p.clientReq.ClientID, []byte(msg))
	if errPublish != nil {
		log.Error("Couldn't connect: ", errPublish)
		return err
	}
	producerPointer.Stop()
	return nil
}

func (p *Params) addCoordinatesToStruct(data []byte) error {

	location := Req{}
	if err := json.Unmarshal(data, &location); err != nil {
		return err
	}
	p.clientReq = location

	return nil
}

func (p *Params) getLocationFromNominatim() (*Nominatim.DataWithoutDetails, error) {

	sqlOpenStr := "dbname=" + p.config.DBname +
		" host=" + p.config.Host +
		" user=" + p.config.User +
		" password=" + p.config.Password

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		log.Critical(err)
		os.Exit(1)
	}
	defer reverseGeocode.Close()

	//oReverseGeocode.SetLanguagePreference()
	reverseGeocode.SetIncludeAddressDetails(p.addressDetails)
	reverseGeocode.SetZoom(p.clientReq.Zoom)
	reverseGeocode.SetLocation(p.clientReq.Lat, p.clientReq.Lon)
	place, err := reverseGeocode.Lookup()
	if err != nil {
		return nil, err
	}

	return place, nil
}

func getLocationJSON(data Nominatim.DataWithoutDetails) (string, error) {

	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return string(dataJSON), nil
}
