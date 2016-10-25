package main

//important: must execute first; do not move
import (
	"fmt"
	"time"

	_ "github.com/KristinaEtc/slflog"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib/utils/request"
	"github.com/go-stomp/stomp"
	"github.com/ventu-io/slf"
)

var log = slf.WithContext("watcher.go")

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

type Process struct {
	connSend  *stomp.Conn
	connSubsc *stomp.Conn
	sub       *stomp.Subscription
	reqIDs    chan string
}

type NecessaryFields struct {
	ID string `json:"id"`
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

	// checking requests: if timeout - increase the err value and delete it
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
	var errOthers int

	go func() {

		for {
			msg, err := pr.sub.Read()
			if err != nil {
				log.Warnf("Error while reading from subcstibtion: %s", err.Error())
				errOthers++
				continue
			}
			gotAnswerFromSub <- msg.Body
		}
	}()

	var IDs []string
	var numOfReq, errTimeOut int

	for {
		select {

		case msg := <-gotAnswerFromSub:

			message := string(msg)
			//if msgCount%globalOpt.Global.MessageDumpInterval == 0 {
			log.Infof("Got message: %s", message)
			id, err := parseID(msg)
			if err != nil {
				log.Error(err.Error())
				errOthers++
			}
			exist, key := containsInSlice(IDs, id.ID)
			if !exist {
				log.Warnf("Got message with wrong id: [%s]", id.ID)
				errOthers++
				continue
			}
			IDs = append(IDs[:key], IDs[key+1:]...)

		case id := <-pr.reqIDs:
			IDs = append(IDs, id)

		case t := <-doneCheckIDs:
			log.Debugf("Ticker ticked %v", t)

			for key, id := range IDs {
				timeWasSended, err := getTimeFromID(id)
				if err != nil {
					log.Errorf("Parsing id: %s", err.Error())
					errOthers++
					continue
				}

				duration := time.Since(*timeWasSended)
				if duration.Seconds() >= 60 {
					log.Warnf("timeout for %s", id)
					errTimeOut++
					IDs = append(IDs[:key], IDs[key+1:]...)
				}
			}

		case _ = <-doneChanResponse:

			reqInJSON, err := createReqBody(IDs, numOfReq)
			if err != nil {
				log.Errorf("Failed to create request to server: [%s]", err.Error())
				errOthers++
				continue
			}

			err = pr.connSend.Send(config.MonitoringTopic, "text/json", []byte(*reqInJSON), nil...)
			if err != nil {
				log.Errorf("Failed to send to server: [%s]", err.Error())
				errOthers++
				continue
			}

			if errTimeOut != 0 {
				err = pr.connSend.Send(config.AlertTopic, "text/json", []byte(*reqInJSON), nil...)
				if err != nil {
					log.Errorf("Failed to send to server: [%s]", err.Error())
					errOthers++
					continue
				}
			}

			numOfReq = 0
			errTimeOut = 0
			errOthers = 0
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
