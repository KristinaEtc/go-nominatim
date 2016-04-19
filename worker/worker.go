package main

import (
	"encoding/json"
	//"Nominatim/lib"
	"database/sql"
	//z"encoding/json"
	"flag"
	"github.com/bitly/go-nsq"
	"sync"
	//"fmt"
	//"bufio"
	_ "github.com/lib/pq"
	"log"
	"os"
	"strconv"
	// "testing"
	//"fmt"
	"strings"
)

var configFile = "../cli/config.json"

//var testFile = "test.csvt"

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
	// speedTest      bool
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

		msgParted := strings.Split(rawMsg, ",")
		params.locParams.lat, err = strconv.ParseFloat(msgParted[0], 32)
		if err != nil {
			log.Print(err)
			return nil
		}
		params.locParams.lon, err = strconv.ParseFloat(msgParted[1], 32)
		if err != nil {
			log.Print(err)
			return nil
		}

		params.locParams.zoom, err = strconv.Atoi(msgParted[2])
		if err != nil {
			log.Print(err)
			return nil
		}

		log.Printf("-%f-%f-%d-", params.locParams.lat, params.locParams.lon, params.locParams.zoom)

		//wg.Done()
		return nil
	}))

	err = consumerPointer.ConnectToNSQD(HOST + PORT)
	if err != nil {
		log.Panic("Could not connect")
	}
	wg.Wait()

}
