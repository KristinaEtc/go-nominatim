package main

//important: must execute first; do not move
import (
	"fmt"
	"time"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib/utils/request"
	_ "github.com/KristinaEtc/slflog"
	"github.com/go-stomp/stomp"
	"github.com/ventu-io/slf"
)

var log = slf.WithContext("main.go")

//var Conns []*stomp.Conn

/*-------------------------
  Config option structures
-------------------------*/

var configFile string

type ServerConf struct {
	ServerAddr       string
	ServerUser       string
	ServerPassword   string
	RequestQueueName string
	ReplyQueuePrefix string
	ClientID         string
	Heartbeat        int
	//	RespondFreq    int
	RequestFreq int // per second
	conn        *stomp.Conn
}

// ConfFile is a file with all program options
type ConfFile struct {
	Server ServerConf
}

var globalOpt = ConfFile{
	Server: ServerConf{
		//RespondFreq: 60,
		RequestFreq:      5,
		ServerAddr:       "localhost:61614",
		ServerUser:       "guest",
		ServerPassword:   "guest",
		ReplyQueuePrefix: "/queue/",
		RequestQueueName: "/queue/nominatimRequest",
		ClientID:         "clientID",
		Heartbeat:        30,
		//	RespondFreq    int
	}}

var options = []func(*stomp.Conn) error{
//stomp.ConnOpt.Login(globalOpt.Global.ServerUser, globalOpt.Global.ServerPassword),
//stomp.ConnOpt.Host(globalOpt.Global.ServerAddr),
}

var (
	stop = make(chan bool)
)

func connect(config *ServerConf) error {
	//heartbeat := time.Duration(globalOpt.Global.Heartbeat) * time.Second
	log.Infof("connect: %+v", config)

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(config.ServerUser, config.ServerPassword),
		stomp.ConnOpt.Host(config.ServerAddr),
		//stomp.ConnOpt.HeartBeat(heartbeat, heartbeat),
	}

	var err error
	config.conn, err = stomp.Dial("tcp", config.ServerAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return err
	}
	return nil
}

func sendMessages(config *ServerConf) {
	defer func() {
		stop <- true
	}()

	//connSend, err := Connect()
	//if err != nil {
	//	log.Errorf("[%v]: %s", node, err.Error() )
	//	return
	//}

	for {

		var i int64 = 0

		ticker := time.NewTicker(time.Second * time.Duration(config.RequestFreq))

		for t := range ticker.C {
			reqAddr := request.GenerateAddress()
			id := fmt.Sprintf("%d,%d", i, t.UnixNano()/1000000)
			i++
			reqInJSON, err := request.MakeReq(reqAddr, config.ClientID, id)
			//reqInJSON, err := request.MakeReq(locs, clientID, i, log)
			if err != nil {
				log.Errorf("Error parse request parameters \"%v\"", err)
				continue
			}

			err = config.conn.Send(config.RequestQueueName, "text/json", []byte(*reqInJSON), nil...)
			if err != nil {
				log.Errorf("Failed to send to server [%v]: %v", config, err)
				continue
			}
		}

	}

}

func recvMessages(subscribed chan bool, node *ServerConf) {
	defer func() {
		stop <- true
	}()

	queueName := node.ReplyQueuePrefix + node.ClientID
	log.Debugf("Subscribing to %s", queueName)

	sub, err := node.conn.Subscribe(queueName, stomp.AckAuto)
	if err != nil {
		log.Errorf("Cannot subscribe to %s: %v", queueName, err.Error())
		return
	}
	close(subscribed)

	var msgCount = 0
	for {

		msg, err := sub.Read()
		if err != nil {
			log.Warn("Got empty message; ignore")
			//time.Sleep(time.Second)
			continue
		}
		//time.Sleep(time.Second)
		message := string(msg.Body)
		//if msgCount%globalOpt.Global.MessageDumpInterval == 0 {
		log.Infof("Got message: %s", message)
		//time.Sleep(time.Second)
		//}
		msgCount++
	}
}

func main() {

	//log = slf.WithContext("main.go")

	subscribed := make(chan bool)

	config.ReadGlobalConfig(&globalOpt, "monitoring options")

	log.Info("connecting...")
	err := connect(&globalOpt.Server)
	if err != nil {
		log.Fatalf("connect: %s", err.Error())
	}

	log.Info("starting working...")

	go recvMessages(subscribed, &globalOpt.Server)

	// wait until we know the receiver has subscribed
	<-subscribed

	go sendMessages(&globalOpt.Server)

	<-stop
	<-stop
}
