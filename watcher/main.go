package main

//important: must execute first; do not move
import (
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/KristinaEtc/slflog"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib/monitoring"
	"github.com/KristinaEtc/go-nominatim/lib/utils/request"
	"github.com/go-stomp/stomp"
	"github.com/ventu-io/slf"
)

var log = slf.WithContext("watcher.go")
var requestTimeOut int64 = 60

var (
	// BuildDate - binary build date
	BuildDate string
	// GitCommit - git commit hash
	GitCommit string
	// GitBranch - git branch name
	GitBranch string
	// GitState - working directory state (clean/dirty)
	GitState string
	// GitSummary - commit summary
	GitSummary string
	// Version - file version
	Version string
)

/*-------------------------
  Config option structures
-------------------------*/

var configFile string

//var uuid string

// ServerConf stores all config information about connection
type ServerConf struct {
	ServerAddr       string
	ServerUser       string
	ServerPassword   string
	RequestQueueName string
	ReplyQueuePrefix string
	AlertTopic       string
	MonitoringTopic  string
	ClientID         string
	Heartbeat        int
	//	RespondFreq    int
	RespondFreq int // per minute
	RequestFreq int // per second
	Name        string
}

// ConfFile is a file with all program options
type ConfFile struct {
	Server      ServerConf
	DirWithUUID string
}

var globalOpt = ConfFile{
	Server: ServerConf{
		RequestFreq:      5,
		ServerAddr:       "localhost:61614",
		ServerUser:       "guest",
		ServerPassword:   "guest",
		ReplyQueuePrefix: "/queue/",
		RequestQueueName: "/queue/nominatimRequest",
		AlertTopic:       "/topic/alerts",
		MonitoringTopic:  "/topic/global_logss",
		ClientID:         "clientUUID",
		Heartbeat:        30,
		RespondFreq:      1,
		Name:             "userName",
	},
	DirWithUUID: ".watcher/",
}

/*-------------------------
  Main process strunctures
-------------------------*/

// Process is a struct with entities
// which provide stomp-interaction
type Process struct {
	connSend         *stomp.Conn         //connection for sendind messages
	connSubsc        *stomp.Conn         //connection for subscribing
	sub              *stomp.Subscription //subscribtion entity
	chGotAddrRequest chan string         //a channel for sending message's id
}

// WatcherData stores data, that will be sended to a topic
type WatcherData struct {
	*monitoring.MonitoringData
	ResponseDelaysByID map[string]int64 `json:"responseDelaysByID"`
	ErrTimeOut         int64
	CurrErrTimeOut     int64
	CurrErrResponses   int64
	CurrSuccResponses  int64
	CurrRequests       int64
	CurrErrorCount     int64
	CurrLastError      string
}

//init in connetc()
var options = []func(*stomp.Conn) error{}

var stop = make(chan bool)

func sendMessages(config ServerConf, pr Process) {

	defer func() {
		stop <- true
	}()

	// Every config.RequestFreq seconds function creates a request to a server
	// with generated address, sends request and sends id of this request
	// to channel reqIDs, which will be readed in recvMessages function.
	for {

		var i int64

		ticker := time.NewTicker(time.Second * time.Duration(config.RequestFreq))

		for t := range ticker.C {
			reqAddr := request.GenerateAddress()
			id := fmt.Sprintf("%d,%d", i, t.UTC().UnixNano())

			i++
			reqInJSON, err := request.MakeReq(reqAddr, globalOpt.Server.ClientID, id)
			if err != nil {
				log.Errorf("Error parse request parameters: [%v]", err)
				continue
			}

			err = pr.connSend.Send(config.RequestQueueName, "application/json", []byte(*reqInJSON), nil...)
			if err != nil {
				log.Errorf("Failed to send to server: [%v]", err)
				continue
			}
			pr.chGotAddrRequest <- id
		}
	}
}

func processMessages(config ServerConf, pr Process) {
	defer func() {
		stop <- true
	}()

	data := WatcherData{}
	data.MonitoringData = monitoring.InitMonitoringData(
		globalOpt.Server.ServerAddr,
		Version,
		globalOpt.Server.Name,
		globalOpt.Server.ClientID,
	)

	data.LastReconnect = data.StartTime
	data.LastError = ""

	// checking requests every second
	tickerCheckRequestsTimeOut := time.NewTicker(time.Second * 1)
	checkRequestsTimeOut := make(chan time.Time)

	go func() {
		for t := range tickerCheckRequestsTimeOut.C {
			checkRequestsTimeOut <- t
		}
	}()

	// sending statistics
	tickerSendDelayStat := time.NewTicker(time.Second * 10)
	sendDelayStatus := make(chan time.Time)

	go func() {
		for t := range tickerSendDelayStat.C {
			sendDelayStatus <- t
		}
	}()

	// got answer from subscription
	// separate goroutine is created, because in direct channel processing
	// there are no possibility to check errors
	chGotAddrResponse := make(chan []byte)
	go func() {
		for {
			time.Sleep(time.Second * 1)
			msg, err := pr.sub.Read()
			if err != nil {
				log.Warnf("Error while reading from subcstibtion: %s", err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}
			//log.Debug("pr.sub.Read()")
			chGotAddrResponse <- msg.Body
		}
	}()

	//--------
	// select loop
	// where the whole logic is implemented

	var timeRequestsByID = make(map[int]int64)
	var responseDelaysByID = make(map[string]int64)

	for {
		select {

		case msg := <-chGotAddrResponse:

			message := string(msg)
			log.Debugf("Address response=[%s]", message)

			requestID, requestTime, workerID, err := parseRequest(msg)
			if err != nil {
				log.Error(err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}

			if _, ok := timeRequestsByID[requestID]; !ok {
				log.Warnf("No requests was sended with such id: [%d,%d]", requestID, requestTime)
				log.Warnf(" timeRequestsByID: [%v]", timeRequestsByID)

				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = fmt.Sprintf("No requests was sended with such id: [%d,%d]", requestID, requestTime)
				data.CurrLastError = fmt.Sprintf("No requests was sended with such id: [%d,%d]", requestID, requestTime)
				continue
			}

			t := time.Now().UTC().UnixNano()
			log.Debugf("t.UTC().Unix() - timeRequestsByID[requestID]=[%d]", (t-timeRequestsByID[requestID])/1000000)
			responseDelaysByID[workerID] = (t - timeRequestsByID[requestID]) / 1000000

			delete(timeRequestsByID, requestID)
			data.CurrSuccResponses++
			data.SuccResp++

		case id := <-pr.chGotAddrRequest:

			//id is a string with  requestID and requestTime: "id,time"
			requestID, requestTime, err := parseRequestID(id)
			if err != nil {
				log.Error(err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}
			timeRequestsByID[requestID] = requestTime
			data.CurrRequests++
			data.Reqs++

		case t := <-checkRequestsTimeOut:
			//log.Debug("CheckIDs")

			for requestID, requestTime := range timeRequestsByID {

				delay := (t.UTC().UnixNano() - requestTime) / 1000000
				if delay >= (requestTimeOut * 1000) {
					log.Warnf("timeout for: [%d,%d]", requestID, requestTime)
					data.ErrorCount++
					data.LastError = fmt.Sprintf("TimeOut for: [%d,%d]", requestID, requestTime)
					data.ErrTimeOut++
					data.CurrErrTimeOut++
					data.CurrLastError = fmt.Sprintf("TimeOut for: [%d,%d]", requestID, requestTime)
					delete(timeRequestsByID, requestID)
				}
			}

		case _ = <-sendDelayStatus:

			//log.Debug("sendStatusMSg")

			data.ResponseDelaysByID = responseDelaysByID
			data.CurrentTime = time.Now().UTC().Format(time.RFC3339)
			data.Subtype = "watcher"

			//log.Debugf("data Map=%v", data.IDs)
			data.CurrentTime = time.Now().Format(time.RFC3339)

			reqInJSON, err := json.Marshal(data)
			if err != nil {
				log.Errorf("Failed to create request to server: [%s]", err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}

			//log.Debugf("Status Message=%s", reqInJSON)

			err = pr.connSend.Send(config.MonitoringTopic, "application/json", []byte(reqInJSON), nil...)
			if err != nil {
				log.Errorf("Failed to send to server: [%s]", err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}

			if data.CurrErrTimeOut != 0 {
				log.Debug("Sending alert message...")
				err = pr.connSend.Send(config.AlertTopic, "application/json", []byte(reqInJSON), nil...)
				if err != nil {
					log.Errorf("Failed to send to server: [%s]", err.Error())
					data.ErrorCount++
					data.CurrErrorCount++
					data.LastError = err.Error()
					data.CurrLastError = err.Error()
					continue
				}
			}

			data.CurrErrorCount = 0
			data.CurrErrResponses = 0
			data.CurrSuccResponses = 0
			data.CurrErrTimeOut = 0
			data.CurrLastError = ""
			data.CurrRequests = 0

			data.ResponseDelaysByID = make(map[string]int64)
			responseDelaysByID = make(map[string]int64)
		}
	}
}

func subscribe(node ServerConf, pr *Process) (err error) {

	queueName := node.ReplyQueuePrefix + node.ClientID
	log.Debugf("Subscribing to %s", queueName)

	pr.sub, err = pr.connSubsc.Subscribe(queueName, stomp.AckAuto)
	if err != nil {
		log.Errorf("Cannot subscribe to %s: %v", queueName, err.Error())
		return
	}
	return
}

func connect(config ServerConf, pr *Process) error {
	//heartbeat := time.Duration(globalOpt.Global.Heartbeat) * time.Second
	log.Infof("connect: %+v", config)

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(config.ServerUser, config.ServerPassword),
		stomp.ConnOpt.Host(config.ServerAddr),
		stomp.ConnOpt.Header("wormmq.link.peer_name", globalOpt.Server.Name),
		stomp.ConnOpt.Header("wormmq.link.peer", globalOpt.Server.ClientID),
	}

	var err error
	pr.connSubsc, err = stomp.Dial("tcp", config.ServerAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return err
	}

	pr.connSend, err = stomp.Dial("tcp", config.ServerAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return err
	}
	return nil
}

func main() {

	log.Error("----------------------------------------------")

	log.Infof("BuildDate=%s\n", BuildDate)
	log.Infof("GitCommit=%s\n", GitCommit)
	log.Infof("GitBranch=%s\n", GitBranch)
	log.Infof("GitState=%s\n", GitState)
	log.Infof("GitSummary=%s\n", GitSummary)
	log.Infof("VERSION=%s\n", Version)

	config.ReadGlobalConfig(&globalOpt, "watcher options")

	globalOpt.Server.ClientID = config.GetUUID(globalOpt.DirWithUUID)

	r := make(chan string, 1)
	process := Process{chGotAddrRequest: r}

	log.Info("connecting...")
	err := connect(globalOpt.Server, &process)
	if err != nil {
		log.Fatalf("connect: %s", err.Error())
	}

	log.Info("subscribing...")
	err = subscribe(globalOpt.Server, &process)

	log.Info("starting working...")

	go processMessages(globalOpt.Server, process)
	go sendMessages(globalOpt.Server, process)

	<-stop
	<-stop
}
