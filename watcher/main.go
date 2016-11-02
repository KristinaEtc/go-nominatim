package main

//important: must execute first; do not move
import (
	"fmt"
	"time"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib/monitoring"
	"github.com/KristinaEtc/go-nominatim/lib/utils/request"
	"github.com/go-stomp/stomp"
	"github.com/ventu-io/slf"
)

var log = slf.WithContext("watcher.go")
var clientID string
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

// ResponseStatistic stores statistics of request's and response's errors
type ResponseStatistic struct {
	*monitoring.MonitoringData

	ErrTimeOut        int64
	CurrErrTimeOut    int64
	CurrErrResponses  int64
	CurrSuccResponses int64
	CurrRequests      int64
	CurrErrorCount    int64
	CurrLastError     string
}

// ResponseDelays stores map with response, grouped by id
type ResponseDelays struct {
	*monitoring.MonitoringData
	DelaysByID []int64 `json:"delays_by_id"`
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
	var i int64
	for {

		ticker := time.NewTicker(time.Second * time.Duration(config.RequestFreq))

		for t := range ticker.C {
			i = i + 1
			reqAddr := request.GenerateAddress()
			id := fmt.Sprintf("%d,%d", i, t.UTC().UnixNano())

			reqInJSON, err := request.MakeReq(reqAddr, clientID, id)
			if err != nil {
				log.Errorf("Error parse request parameters: [%v]", err)
				continue
			}

			log.Debugf("Req=%s", *reqInJSON)

			err = pr.connSend.Send(config.RequestQueueName, "application/json", []byte(*reqInJSON), nil...)
			if err != nil {
				log.Errorf("Failed to send to server: [%v]", err)
				continue
			}
			pr.chGotAddrRequest <- id
		}
	}
}

func initMonitoringStructures() (ResponseStatistic, ResponseDelays) {
	dataStatistic := ResponseStatistic{}
	dataStatistic.MonitoringData = monitoring.InitMonitoringData(
		globalOpt.Server.ServerAddr,
		Version,
		globalOpt.Server.Name,
		clientID,
	)

	dataStatistic.LastReconnect = dataStatistic.StartTime
	dataStatistic.LastError = ""

	dataDelays := ResponseDelays{}
	dataDelays.MonitoringData = monitoring.InitMonitoringData(
		globalOpt.Server.ServerAddr,
		Version,
		globalOpt.Server.Name,
		clientID,
	)

	return dataStatistic, dataDelays
}

func processMessages(config ServerConf, pr Process) {
	defer func() {
		stop <- true
	}()

	dataStatistic, dataDelays := initMonitoringStructures()

	// checking requests every second
	tickerCheckRequestsTimeOut := time.NewTicker(time.Second * 1)

	// sending statistics
	tickerSendDelayStat := time.NewTicker(time.Second * 10)

	// got answer from subscription
	// separate goroutine is created, because in direct channel processing
	// there are no possibility to check errors
	chGotAddrResponse := make(chan []byte)
	go func() {
		for {
			//time.Sleep(time.Second * 1)
			msg, err := pr.sub.Read()
			if err != nil {
				log.Warnf("Error while reading from subscription: %s", err.Error())
				processCommonError(err.Error(), &dataStatistic)
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
	var responseDelaysByID = make(map[string][]int64)

	for {
		select {

		case msg := <-chGotAddrResponse:

			message := string(msg)
			log.Debugf("Response=[%s]", message)

			requestID, requestTime, workerID, err := parseResponse(msg)
			if err != nil {
				log.Error(err.Error())
				processCommonError(err.Error(), &dataStatistic)
				continue
			}

			timeRequest, ok := timeRequestsByID[requestID]
			if !ok {
				log.Debugf("timeRequestsByID: [%v]", timeRequestsByID)
				errMessage := fmt.Sprintf("No requests was sended with such id: [%d,%d]", requestID, requestTime)
				log.Warnf(errMessage)
				processCommonError(errMessage, &dataStatistic)
				continue
			}

			t := time.Now().UTC().UnixNano()
			responseDelaysByID[workerID] = append(responseDelaysByID[workerID], (t-timeRequest)/1000000)
			//log.Debugf("[%d] t.UTC().Unix() - timeRequestsByID[requestID]=[%d]", requestID, (t-timeRequestsByID[requestID])/1000000)

			delete(timeRequestsByID, requestID)
			processSuccess(&dataStatistic)

		case id := <-pr.chGotAddrRequest:

			//id is a string with  requestID and requestTime: "id,time"
			requestID, requestTime, err := parseID(id)
			if err != nil {
				log.Error(err.Error())
				processCommonError(err.Error(), &dataStatistic)
				continue
			}
			timeRequestsByID[requestID] = requestTime
			processNewRequest(&dataStatistic)

		case t := <-tickerCheckRequestsTimeOut.C:
			//log.Debug("CheckIDs")

			for requestID, requestTime := range timeRequestsByID {

				delay := (t.UTC().UnixNano() - requestTime) / 1000000
				if delay >= requestTimeOut*1000 {
					log.Warnf("timeout for: [%d,%d]", requestID, requestTime)

					errMessage := fmt.Sprintf("TimeOut for: [%d,%d]", requestID, requestTime)
					processErrorTimeOut(errMessage, &dataStatistic)
					delete(timeRequestsByID, requestID)
				}
			}

		case _ = <-tickerSendDelayStat.C:

			//log.Debug("sendStatusMSg")
			dataDelays.CurrentTime = time.Now().UTC().Format(time.RFC3339)
			dataStatistic.Subtype = "watcher-delays"

			for k, v := range responseDelaysByID {
				dataDelays.Subsystem = k
				dataDelays.DelaysByID = v

				err := sendMessageDelays(&dataDelays, &dataStatistic, pr, config.MonitoringTopic)
				if err != nil {
					log.Errorf("Failed to create request to server: [%s]", err.Error())
					processCommonError(err.Error(), &dataStatistic)
					continue
				}
			}

			//log.Debugf("data Map=%v", timeRequestsByID)

			reqInJSON, err := sendMessageStatistic(&dataStatistic, &dataStatistic, pr, config.MonitoringTopic)
			if err != nil {
				log.Errorf("Failed to create request to server: [%s]", err.Error())
				processCommonError(err.Error(), &dataStatistic)
				continue
			}

			if dataStatistic.CurrErrTimeOut != 0 {
				log.Debug("Sending alert message...")
				err = pr.connSend.Send(config.AlertTopic, "application/json", reqInJSON, nil...)
				if err != nil {
					log.Errorf("Failed to send to server: [%s]", err.Error())
					processCommonError(err.Error(), &dataStatistic)
					continue
				}
			}

			cleanErrorStat(&dataStatistic)
			responseDelaysByID = make(map[string][]int64)
		}
	}
}

func subscribe(node ServerConf, pr *Process) (err error) {

	queueName := node.ReplyQueuePrefix + clientID + "-AddressReply"
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
		stomp.ConnOpt.Header("wormmq.link.peer", clientID),
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

	clientID = config.GetUUID(globalOpt.DirWithUUID)

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
