package main

//important: do not move
import (
	"github.com/KristinaEtc/go-nominatim/lib/utils/fileproc"
	"github.com/KristinaEtc/go-nominatim/lib/utils/request"
	_ "github.com/KristinaEtc/slflog"
	u "github.com/KristinaEtc/utils"
	"github.com/go-stomp/stomp"
	"github.com/ventu-io/slf"
)

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
	ServerAddr     string
	ServerUser     string
	ServerPassword string
	QueueFormat    string
	QueueName      string
	DestinQueue    string
	TestFile       string
}

// ConfFile is a file with all program options
type ConfFile struct {
	Global GlobalConf
}

var globalOpt = ConfFile{
	Global: GlobalConf{
		ServerAddr:     "localhost:61614",
		QueueFormat:    "/queue/",
		QueueName:      "/queue/nominatimRequest",
		ServerUser:     "",
		ServerPassword: "",
		TestFile:       "test.csv",
		DestinQueue:    "/queue/nominatimRequest",
	},
}

var options = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login(globalOpt.Global.ServerUser, globalOpt.Global.ServerPassword),
	stomp.ConnOpt.Host(globalOpt.Global.ServerAddr),
}

var (
	stop = make(chan bool)
)

func sendMessages() {
	defer func() {
		stop <- true
	}()

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(globalOpt.Global.ServerUser, globalOpt.Global.ServerPassword),
		stomp.ConnOpt.Host(globalOpt.Global.ServerAddr),
	}

	_, err := stomp.Dial("tcp", globalOpt.Global.ServerAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return
	}

	connSend, err := stomp.Dial("tcp", globalOpt.Global.ServerAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return
	}

	fs, err := fileproc.NewFileScanner(globalOpt.Global.TestFile)
	if err != nil {
		log.Panic(err.Error())
	}
	defer fs.Close()

	fs.Scanner = fs.GetScanner()

	i := 0
	for fs.Scanner.Scan() {
		/*if fileNotFinished := fs.Scanner.Scan(); fileNotFinished == true {*/
		locs := fs.Scanner.Text()

		log.Debugf("locs: %s", locs)

		reqInJSON, err := request.MakeReq(locs, clientID, i)
		//reqInJSON, err := request.MakeReq(locs, clientID, i, log)
		if err != nil {
			log.Error("Could not get coordinates in JSON: wrong format")
			continue
		}

		log.Debugf("reqInJSON: %s", *reqInJSON)

		err = connSend.Send(globalOpt.Global.DestinQueue, "text/json", []byte(*reqInJSON), nil...)
		if err != nil {
			log.Errorf("Failed to send to server: %v", err)
			return
		}
		i++
	}
}

func recvMessages(subscribed chan bool) {
	defer func() {
		stop <- true
	}()

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(globalOpt.Global.ServerUser, globalOpt.Global.ServerPassword),
		stomp.ConnOpt.Host(globalOpt.Global.ServerAddr),
	}

	conn, err := stomp.Dial("tcp", globalOpt.Global.ServerAddr, options...)
	if err != nil {
		log.Errorf("Cannot connect to server: %v", err.Error())
		return
	}

	log.Debugf("Subscribing to %s", globalOpt.Global.QueueFormat+clientID)

	sub, err := conn.Subscribe(globalOpt.Global.QueueFormat+clientID, stomp.AckAuto)
	if err != nil {
		log.Errorf("Cannot subscribe to %s: %v", globalOpt.Global.QueueFormat+clientID, err.Error())
		return
	}
	close(subscribed)

	var msgCount = 0
	for {
		msg := <-sub.C
		if msg == nil {
			log.Warn("Got empty message; ignore")
			return
		}

		message := string(msg.Body)
		if msgCount%20 == 0 {
			log.Infof("Got message: %s", message)
		}
		msgCount++
	}
}

func main() {

	subscribed := make(chan bool)

	u.GetFromGlobalConf(&globalOpt, "go-stomp-client options")

	log.Info("starting working...")

	go recvMessages(subscribed)
	// wait until we know the receiver has subscribed
	<-subscribed

	go sendMessages()

	<-stop
	<-stop
}
