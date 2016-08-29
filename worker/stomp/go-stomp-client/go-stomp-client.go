package main

//important: must execute first; do not move
import (
	"time"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib/utils/fileproc"
	"github.com/KristinaEtc/go-nominatim/lib/utils/request"
	_ "github.com/KristinaEtc/slflog"
	"github.com/go-stomp/stomp"
	"github.com/ventu-io/slf"
)

//  u "github.com/KristinaEtc/utils"

const (
	defaultPort = ":61614"
	clientID    = "clientID"
)

var log = slf.WithContext("go-stomp-client.go")

/*serverAddr  = flag.String("server", "localhost:61614", "STOMP server endpoint")
destination = flag.String("topic", "/queue/nominatimRequest", "Destination topic")
queueFormat = flag.String("queue", "/queue/", "Queue format")
login       = flag.String("login", "client1", "Login for authorization")
passcode    = flag.String("pwd", "111", "Passcode for authorization")
testFile    = flag.String("testfile", "test.csv", "testfile with coordinates")
*/

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
	TestFile            string
	ClientID            string
	MessageDumpInterval int
	Heartbeat           int
}

// ConfFile is a file with all program options
type ConfFile struct {
	Global GlobalConf
}

var globalOpt = ConfFile{
	Global: GlobalConf{
		ServerAddr:          "localhost:61614",
		QueueFormat:         "/queue/",
		QueueName:           "/queue/nominatimRequest",
		ServerUser:          "",
		ServerPassword:      "",
		TestFile:            "test.csv",
		ClientID:            "clientID",
		MessageDumpInterval: 20,
		Heartbeat:           30,
	},
}

var options = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login(globalOpt.Global.ServerUser, globalOpt.Global.ServerPassword),
	stomp.ConnOpt.Host(globalOpt.Global.ServerAddr),
}

var (
	stop = make(chan bool)
)

func Connect() (*stomp.Conn, error) {
	heartbeat := time.Duration(globalOpt.Global.Heartbeat) * time.Second

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(globalOpt.Global.ServerUser, globalOpt.Global.ServerPassword),
		stomp.ConnOpt.Host(globalOpt.Global.ServerAddr),
		stomp.ConnOpt.HeartBeat(heartbeat, heartbeat),
	}

	conn, err := stomp.Dial("tcp", globalOpt.Global.ServerAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return nil, err
	}
	return conn, nil
}

func sendMessages() {
	defer func() {
		stop <- true
	}()

	if globalOpt.Global.TestFile == "" {
		log.Infof("No test file, stopping sendMessages")
		return
	}

	connSend, err := Connect()
	if err != nil {
		return
	}

	fs, err := fileproc.NewFileScanner(globalOpt.Global.TestFile)
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer fs.Close()

	fs.Scanner = fs.GetScanner()

	i := 0
	for fs.Scanner.Scan() {
		/*if fileNotFinished := fs.Scanner.Scan(); fileNotFinished == true {*/
		locs := fs.Scanner.Text()

		//log.Infof("File line: %s", locs)

		reqInJSON, err := request.MakeReq(locs, globalOpt.Global.ClientID, i)
		//reqInJSON, err := request.MakeReq(locs, clientID, i, log)
		if err != nil {
			log.Errorf("Error parse request parameters \"%v\"", err)
			continue
		}

		//log.Infof("Request: %s", *reqInJSON)

		err = connSend.Send(globalOpt.Global.QueueName, "text/json", []byte(*reqInJSON), nil...)
		if err != nil {
			log.Errorf("Failed to send to server: %v", err)
			continue
		}
		i++
	}
}

func recvMessages(subscribed chan bool) {
	defer func() {
		stop <- true
	}()

	conn, err := Connect()
	if err != nil {
		return
	}

	queueName := globalOpt.Global.QueueFormat + globalOpt.Global.ClientID
	log.Debugf("Subscribing to %s", queueName)

	sub, err := conn.Subscribe(queueName, stomp.AckAuto)
	if err != nil {
		log.Errorf("Cannot subscribe to %s: %v", queueName, err.Error())
		return
	}
	close(subscribed)

	var msgCount = 0
	for {

		msg, err := sub.Read()
		if err != nil {
			//log.Warn("Got empty message; ignore")
			time.Sleep(time.Second)
			continue
		}
		time.Sleep(time.Second)
		message := string(msg.Body)
		//if msgCount%globalOpt.Global.MessageDumpInterval == 0 {
		log.Infof("Got message: %s", message)
		time.Sleep(time.Second)
		//}
		msgCount++
	}
}

func main() {

	subscribed := make(chan bool)

	config.ReadGlobalConfig(&globalOpt, "go-stomp-client options")

	log.Info("starting working...")

	go recvMessages(subscribed)
	// wait until we know the receiver has subscribed
	<-subscribed

	go sendMessages()

	<-stop
	<-stop
}
