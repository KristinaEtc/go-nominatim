package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"github.com/bitly/go-nsq"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	ClientID           string = "client1"
	HOST               string = "localhost"
	PORT               string = ":4150"
	topicToSubscribe   string = "main"
	channelToSubscribe string = "main"
)

var (
	numOfReq  = 10000
	speedTest = false
	testFile  = "test.csv"
)

type Req struct {
	Lat      float64 `json: Lat`
	Lon      float64 `json:Lon`
	Zoom     int     `json:Zoom`
	ClientID string  `json:ClientID`
}

func (r *Req) getParams() {

	flag.Parsed()

	flag.Float64Var(&r.Lat, "a", 53.90223, "lat")
	flag.Float64Var(&r.Lon, "b", 27.56191, "log")
	flag.IntVar(&r.Zoom, "z", 18, "zoom")
	//flag.BoolVar(&speedTest, "t", true, "speed test")
	flag.StringVar(&(testFile), "n", testFile, "name of your test file")

	flag.Parse()

	return
}

func (r *Req) getLocationJSON() (string, error) {

	dataJSON, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(dataJSON), nil
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var reqInJSON []string

	r := Req{ClientID: ClientID}
	r.getParams()
	log.Println(r)

	speedTest = true

	if speedTest == false {
		log.Println("speedTest == false\n\n")
		numOfReq = 1

		jsonReq, err := r.getLocationJSON()
		if err != nil {
			log.Println(err)
			return
		}
		reqInJSON = append(reqInJSON, jsonReq)
		log.Println(jsonReq)

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
			r := Req{}
			r.Lat, err = strconv.ParseFloat(locSlice[0], 32)
			if err != nil {
				log.Print(err)
				continue
			}

			r.Lon, err = strconv.ParseFloat(locSlice[1], 32)
			if err != nil {
				log.Print(err)
				continue
			}
			r.Zoom, err = strconv.Atoi(locSlice[2])
			if err != nil {
				log.Print(err)
				continue
			}
			r.ClientID = ClientID

			jsonReq, err := r.getLocationJSON()
			if err != nil {
				log.Println(err)
				return
			}
			reqInJSON = append(reqInJSON, jsonReq)
		}
	}

	wg := &sync.WaitGroup{}

	listener(wg)

	config := nsq.NewConfig()
	producerPointer, err := nsq.NewProducer(HOST+PORT, config)
	if err != nil {
		log.Println("Couldn't create new worker: ", err)
		os.Exit(1)
	}

	var errors, found = 0, 0

	for _, data := range reqInJSON {

		errPublish := producerPointer.Publish(topicToSubscribe, []byte(data))
		if errPublish != nil {
			log.Println("Couldn't connect: ", errPublish)

			errors++
			continue
		}
		wg.Add(1)
		found++
	}
	producerPointer.Stop()
	wg.Wait()

	log.Println("found:", found, "errors:", errors)
}

func listener(wg *sync.WaitGroup) {

	config := nsq.NewConfig()
	q, err := nsq.NewConsumer(ClientID, ClientID, config)
	if err != nil {
		log.Println("Could not create client: ", err)
	}
	q.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		rawMsg := string(message.Body[:len(message.Body)])

		log.Printf("Got a message: %v, %s", rawMsg, message.NSQDAddress)
		wg.Done()
		return nil
	}))
	err = q.ConnectToNSQD(HOST + PORT)
	if err != nil {
		log.Panic("Could not connect")
	}

}
