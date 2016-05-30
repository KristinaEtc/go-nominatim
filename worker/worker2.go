package main

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	l4g "github.com/alecthomas/log4go"
	//"github.com/bitly/go-nsq"
	"github.com/go-stomp/stomp"
	_ "github.com/lib/pq"
	"os"
	//"sync"
)

var configFile = "config.json"

const defaultPort = ":61614"

var serverAddr = flag.String("server", "localhost:61614", "STOMP server endpoint")
var messageCount = flag.Int("count", 10, "Number of messages to send/receive")
var queueName = flag.String("queue", "/queue/nominatimRequest", "Destination queue")
var helpFlag = flag.Bool("help", false, "Print help text")
var stop = make(chan bool)

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

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("guest", "guest"),
	stomp.ConnOpt.Host("/"),
}

type ConfigDB struct {
	DBname, Host, User, Password string
}

const (
	HOST               string = "localhost"
	PORT               string = ":4150"
	topicToSubscribe   string = "main"
	channelToSubscribe string = "main"
	LOGFILE            string = "go-worker.log"
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
	log.Info("db configurate done")
}

func (p *Params) locationSearch(rawMsg []byte) ([]byte, *string, error) {

	log.Debug("Got request: %s", rawMsg)

	err := p.addCoordinatesToStruct(rawMsg)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	log.Debug("addCoordinatesToStruct done")
	place, err := p.getLocationFromNominatim()
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}

	log.Debug("getLocationFromNominatim done")
	placeJSON, err := getLocationJSON(*place)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	log.Debug("etLocationJSON done")

	whoToSent := p.clientReq.ClientID

	return placeJSON, &whoToSent, nil

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

func getLocationJSON(data Nominatim.DataWithoutDetails) ([]byte, error) {

	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return dataJSON, nil
}

func requestLoop(subscribed chan bool, p *Params) {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)

	if err != nil {
		println("cannot connect to server", err.Error())
		return
	}

	sub, err := conn.Subscribe(*queueName, stomp.AckAuto)
	if err != nil {
		println("cannot subscribe to", *queueName, err.Error())
		return
	}
	close(subscribed)

	for {
		msg := <-sub.C
		if msg == nil {
			log.Error("Got nil message")
			return
		}

		reqJSON, whoToSent, err := p.locationSearch(msg.Body)
		if err != nil {
			log.Error("Error: converting request to json")
			return
		}

		log.Info("whoToSent %s", *whoToSent)

		err = conn.Send("/queue/"+*whoToSent, "text/plain",
			[]byte(reqJSON), nil...)
		if err != nil {
			println("failed to send to server", err)
			return
		}
		log.Info("Sending finished")
	}
}

func main() {

	log = l4g.NewLogger()

	log.AddFilter("stdout", l4g.INFO, l4g.NewConsoleLogWriter())
	log.AddFilter("file", l4g.DEBUG, l4g.NewFileLogWriter(LOGFILE, true))

	params := Params{}
	params.configurateDB()
	subscribed := make(chan bool)
	go requestLoop(subscribed, &params)

	<-stop

	//params.getParams()
	//params.locationSearch()
}
