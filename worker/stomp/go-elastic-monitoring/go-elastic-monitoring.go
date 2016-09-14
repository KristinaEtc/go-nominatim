package main

import (
	//important: must execute first; do not move
	"strconv"

	_ "github.com/KristinaEtc/slflog"
	elastic "gopkg.in/olivere/elastic.v3"

	"sync"
	"time"

	"github.com/KristinaEtc/config"
	"github.com/go-stomp/stomp"
	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"
)

var log = slf.WithContext("go-stomp-nominatim.go")

var options = []func(*stomp.Conn) error{}

var (
	stop   = make(chan bool)
	client *elastic.Client
)

var (
	// These fields are populated by govvv
	BuildDate  string
	GitCommit  string
	GitBranch  string
	GitState   string
	GitSummary string
	Version    string
)

type Subs struct {
	Host     string
	Login    string
	Passcode string
	Queue    string
	Index    string
}

// GlobalConf is a struct with global options,
// like server address and queue format, etc.

type ConfigFile struct {
	Subscriptions []Subs
}

var globalOpt = ConfigFile{
	Subscriptions: []Subs{},
}

func recvMessages() {

	defer func() {
		stop <- true
	}()

	var wg sync.WaitGroup
	for _, sub := range globalOpt.Subscriptions {
		wg.Add(1)
		go readFromSub(sub, &wg)
	}
	wg.Wait()
}

func Connect(address string, login string, passcode string) (*stomp.Conn, error) {
	//defer func() {
	//	stop <- true
	//}()
	//heartbeat := time.Duration(globalOpt.Global.Heartbeat) * time.Second

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(login, passcode),
		stomp.ConnOpt.Host(address),
		//stomp.ConnOpt.HeartBeat(heartbeat, heartbeat),
	}

	log.Infof("Connecting to %s: [%s, %s]\n", address, login, passcode)

	conn, err := stomp.Dial("tcp", address, options...)
	if err != nil {
		log.Errorf("cannot connect to server %s: %v", address, err.Error())
		return nil, err
	}

	return conn, nil
}

func readFromSub(subNode Subs, wg *sync.WaitGroup) {
	var msgCount = 0
	defer wg.Done()

	conn, err := Connect(subNode.Host, subNode.Login, subNode.Passcode)
	if err != nil {
		return
	}
	log.Infof("Subscribing to %s", subNode.Queue)
	sub, err := conn.Subscribe(subNode.Queue, stomp.AckAuto)
	if err != nil {
		log.Errorf("Cannot subscribe to %s: %v", subNode.Queue, err.Error())
		return
	}

	for {
		msg, err := sub.Read()
		if err != nil {
			//log.Warn("Got empty message; ignore")
			time.Sleep(time.Second)
			continue
		}
		time.Sleep(time.Second)
		message := string(msg.Body)
		log.Infof("[%s]/[%s]", subNode.Queue, subNode.Index)
		_, err = client.Index().
			Index(subNode.Index).
			Type("external").
			Id(strconv.Itoa(msgCount)).
			BodyJson(message).
			Refresh(true).
			Do()
		if err != nil {
			log.Errorf("Elasticsearch: %s", err.Error())
		}
		time.Sleep(time.Second)
		msgCount++
	}
}

func main() {

	var err error
	config.ReadGlobalConfig(&globalOpt, "go-elastic-monitoring options")
	client, err = elastic.NewClient()
	if err != nil {
		log.Error("elasicsearch: could not create client")
	}

	log.Infof("BuildDate=%s\n", BuildDate)
	log.Infof("GitCommit=%s\n", GitCommit)
	log.Infof("GitBranch=%s\n", GitBranch)
	log.Infof("GitState=%s\n", GitState)
	log.Infof("GitSummary=%s\n", GitSummary)
	log.Infof("VERSION=%s\n", Version)

	log.Info("Starting working...")

	go recvMessages()
	<-stop
}
