package main

//important: must execute first; do not move
import (
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
var uuid string

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
		MonitoringTopic:  "/topic/global_logs",
		ClientID:         "clientID",
		Heartbeat:        30,
		RespondFreq:      1,
	},
	DirWithUUID: ".go-stomp-nominatim/",
}

/*-------------------------
  Main process strunctures
-------------------------*/

// Process is a struct with entities
// which provide stomp-interaction
type Process struct {
	connSend  *stomp.Conn         //connection for sendind messages
	connSubsc *stomp.Conn         //connection for subscribing
	sub       *stomp.Subscription //subscribtion entity
	reqIDs    chan string         //a channel for sending message's id
}

type NecessaryFields struct {
	ID string `json:"id"`
}

type WatcherData struct {
	*monitoring.MonitoringData
	IDs               []string `json:"response_stat"`
	ErrTimeOut        int64
	CurrErrTimeOut    int64
	CurrErrResponses  int64
	CurrSuccResponses int64
	CurrRequests      int64
	CurrErrorCount    int64
	CurrLastError     string
}

var options = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login(globalOpt.Server.ServerUser, globalOpt.Server.ServerPassword),
	stomp.ConnOpt.Host(globalOpt.Server.ServerAddr),
}

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
			id := fmt.Sprintf("%d,%d", i, t.UnixNano()/1000000)
			i++
			reqInJSON, err := request.MakeReq(reqAddr, config.ClientID, id)
			if err != nil {
				log.Errorf("Error parse request parameters: [%v]", err)
				continue
			}

			err = pr.connSend.Send(config.RequestQueueName, "text/json", []byte(*reqInJSON), nil...)
			if err != nil {
				log.Errorf("Failed to send to server: [%v]", err)
				continue
			}
			pr.reqIDs <- id
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
		globalOpt.Server.ClientID,
		uuid,
	)

	// checking requests:
	// if timeout - increase the err value and delete it
	tickerCheckIDs := time.NewTicker(time.Second * 1)
	doneCheckIDs := make(chan time.Time)

	go func() {
		for t := range tickerCheckIDs.C {
			doneCheckIDs <- t
		}
	}()

	// sending statistics to spetial topic(s)
	tickerResp := time.NewTicker(time.Second * 10)
	doneChanResponse := make(chan time.Time)

	go func() {
		for t := range tickerResp.C {
			doneChanResponse <- t
		}
	}()

	// got answer from subscription
	// separate goroutine is created, because in direct channel processing
	// there are no possibility to check error
	// case msg := sub.C
	gotAnswerFromSub := make(chan []byte)

	go func() {

		for {
			msg, err := pr.sub.Read()
			if err != nil {
				log.Warnf("Error while reading from subcstibtion: %s", err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}
			gotAnswerFromSub <- msg.Body
		}
	}()

	var IDs []string

	for {
		select {

		case msg := <-gotAnswerFromSub:

			message := string(msg)
			//if msgCount%globalOpt.Global.MessageDumpInterval == 0 {
			log.Infof("Got message: %s", message)
			id, err := parseID(msg)
			if err != nil {
				log.Error(err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}
			exist, key := containsInSlice(IDs, id.ID)
			if !exist {
				log.Warnf("Got message with wrong id: [%s]", id.ID)
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = fmt.Sprintf("Got message with wrong id: [%s]", id.ID)
				data.CurrLastError = fmt.Sprintf("Got message with wrong id: [%s]", id.ID)
				continue
			}
			IDs = append(IDs[:key], IDs[key+1:]...)
			data.CurrSuccResponses++
			data.SuccResp++

		case id := <-pr.reqIDs:
			IDs = append(IDs, id)
			data.CurrRequests++
			data.Reqs++

		case _ = <-doneCheckIDs:
			//log.Debugf("Ticker ticked %v", t)

			for key, id := range IDs {
				timeWasSended, err := getTimeFromID(id)
				if err != nil {
					data.ErrorCount++
					data.CurrErrorCount++
					data.LastError = err.Error()
					data.CurrLastError = err.Error()
					continue
				}

				duration := time.Since(*timeWasSended)
				if duration.Seconds() >= 60 {
					log.Warnf("timeout for %s", id)
					data.ErrorCount++
					data.LastError = fmt.Sprintf("TimeOut for %s", id)
					data.ErrTimeOut++
					data.CurrErrTimeOut++
					data.CurrLastError = fmt.Sprintf("TimeOut for %s", id)
					IDs = append(IDs[:key], IDs[key+1:]...)
				}
			}

		case _ = <-doneChanResponse:

			data.IDs = IDs
			data.CurrentTime = time.Now().Format(time.RFC3339)
			log.Errorf("data=%v", data.IDs)

			reqInJSON, err := getJSON(data)
			if err != nil {
				log.Errorf("Failed to create request to server: [%s]", err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}

			log.Errorf("reqInJSON=%s", reqInJSON)

			err = pr.connSend.Send(config.MonitoringTopic, "text/json", []byte(reqInJSON), nil...)
			if err != nil {
				log.Errorf("Failed to send to server: [%s]", err.Error())
				data.ErrorCount++
				data.CurrErrorCount++
				data.LastError = err.Error()
				data.CurrLastError = err.Error()
				continue
			}

			if data.CurrErrTimeOut != 0 {
				/*err = pr.connSend.Send(config.AlertTopic, "text/json", []byte(reqInJSON), nil...)
				if err != nil {
					log.Errorf("Failed to send to server: [%s]", err.Error())
					data.ErrorCount++
					data.CurrErrorCount++
					data.LastError = err.Error()
					data.CurrLastError = err.Error()
					continue
				}*/
			}

			data.CurrErrorCount = 0
			data.CurrErrResponses = 0
			data.CurrSuccResponses = 0
			data.CurrErrTimeOut = 0
			data.CurrLastError = ""
			data.CurrRequests = 0

			data.IDs = data.IDs[:0]
			IDs = IDs[:0]
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
		//stomp.ConnOpt.HeartBeat(heartbeat, heartbeat),
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

	uuid = config.GetUUID(globalOpt.DirWithUUID)

	r := make(chan string, 1)
	process := Process{reqIDs: r}

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
