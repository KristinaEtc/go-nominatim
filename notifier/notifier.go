package main

//important: must execute first; do not move
import (
	"encoding/json"
	"errors"
	"time"

	"github.com/KristinaEtc/alert"
	"github.com/KristinaEtc/config"
	_ "github.com/KristinaEtc/slflog"
	"github.com/go-stomp/stomp"
	"github.com/ventu-io/slf"
)

//  u "github.com/KristinaEtc/utils"

const (
	defaultPort = ":61614"
	clientID    = "clientID"
)

var log = slf.WithContext("notifier.go")
var nodeID string

/*-------------------------
  Config option structures
-------------------------*/

var configFile string

// GlobalConf is a struct with global options,
// like server address and queue format, etc.
type GlobalConf struct {
	ServerAddr          string
	ServerUser          string
	ServerPassword      string
	QueueFormat         string
	QueueName           string
	ClientID            string
	MessageDumpInterval int
	Heartbeat           int
	MusicFile           string
	Name                string
}

// ConfFile is a file with all program options
type ConfFile struct {
	Global      GlobalConf
	DirWithUUID string
}

var globalOpt = ConfFile{
	Global: GlobalConf{
		ServerAddr:          "localhost:61614",
		QueueFormat:         "/queue/",
		QueueName:           "/queue/nominatimRequest",
		ServerUser:          "",
		ServerPassword:      "",
		ClientID:            "clientID",
		MessageDumpInterval: 20,
		Heartbeat:           30,
		MusicFile:           "7.aiff",
		Name:                "notifier",
	},
	DirWithUUID: ".notifier/",
}

// NecessaryFields storesrows of json request, that we want to get
// and that should necessary be
type NecessaryFields struct {
	ID string `json:"id"`
}

func getID(msg []byte) (string, error) {

	var data NecessaryFields

	if err := json.Unmarshal(msg, &data); err != nil {
		log.Errorf("Could not parse response: %s", err.Error())
		return "", err
	}
	if data.ID == "" {
		log.Warnf("Messsage with empty ID: %s", string(msg))
		return "", errors.New("No utc value in request")
	}
	return data.ID, nil
}

var options = []func(*stomp.Conn) error{}

var (
	stop = make(chan bool)
)

func Connect() (*stomp.Conn, error) {
	heartbeat := time.Duration(globalOpt.Global.Heartbeat) * time.Second

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(globalOpt.Global.ServerUser, globalOpt.Global.ServerPassword),
		stomp.ConnOpt.Host(globalOpt.Global.ServerAddr),
		stomp.ConnOpt.HeartBeat(heartbeat, heartbeat),
		stomp.ConnOpt.Header("wormmq.link.peer_name", globalOpt.Global.Name),
		stomp.ConnOpt.Header("wormmq.link.peer", nodeID),
	}

	conn, err := stomp.Dial("tcp", globalOpt.Global.ServerAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return nil, err
	}
	return conn, nil
}

func recvMessages() {
	defer func() {
		stop <- true
	}()

	conn, err := Connect()
	if err != nil {
		return
	}

	queueName := globalOpt.Global.QueueName
	log.Debugf("Subscribing to %s", queueName)

	sub, err := conn.Subscribe(queueName, stomp.AckAuto)
	if err != nil {
		log.Errorf("Cannot subscribe to %s: %v", queueName, err.Error())
		return
	}

	var msgCount = 0
	for {

		msg, err := sub.Read()
		if err != nil {
			continue
		}
		message := string(msg.Body)

		log.Infof("Got message: %s", message)
		id, err := getID(msg.Body)
		if err != nil {
			log.Errorf("Parse ID %s: %v", queueName, err.Error())
			return
		}

		go alert.PopUpNotify(id)
		go alert.PlayMusic(globalOpt.Global.MusicFile)

		msgCount++
	}
}

func main() {

	config.ReadGlobalConfig(&globalOpt, "stomp-alert options")
	nodeID = config.GetUUID(globalOpt.DirWithUUID)

	log.Info("starting working...")

	go recvMessages()
	<-stop
}
