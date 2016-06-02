package main

import (
	"Nominatim/lib/utils/fileproc"
	"Nominatim/lib/utils/request"
	"flag"
	l4g "github.com/alecthomas/log4go"
	"github.com/go-stomp/stomp"
	"os"
)

var log l4g.Logger = l4g.NewLogger()

const (
	defaultPort = ":61613"
	clientID    = "clientID"
)

var (
	serverAddr  = flag.String("server", "localhost:61613", "STOMP server endpoint")
	destination = flag.String("topic", "/queue/nominatimRequest", "Destination topic")
	queueFormat = flag.String("queue", "/queue/", "Queue format")
	login       = flag.String("login", "client", "Login for authorization")
	passcode    = flag.String("pwd", "111", "Passcode for authorization")
	testFile    = flag.String("testfile", "../test.csv", "testfile with coordinates")
	LOGFILE     = flag.String("log", "client.log", "logfile path/name")

	debugMode = flag.Bool("debug", false, "Debug mode")
	stop      = make(chan bool)
)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("guest", "guest"),
	stomp.ConnOpt.Host("/"),
}

func init() {
	log.AddFilter("stdout", l4g.INFO, l4g.NewConsoleLogWriter())
	log.AddFilter("file", l4g.DEBUG, l4g.NewFileLogWriter(*LOGFILE, false))
}

func sendMessages() {
	defer func() {
		stop <- true
	}()

	_, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Error("cannot connect to server %v", err.Error())
		return
	}

	connSend, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Error("cannot connect to server %v", err.Error())
		return
	}

	fs, err := fileproc.NewFileScanner(*testFile)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	defer fs.Close()

	fs.Scanner = fs.GetScanner()

	i := 0

	for fs.Scanner.Scan() {
		/*if fileNotFinished := fs.Scanner.Scan(); fileNotFinished == true {*/
		locs := fs.Scanner.Text()

		if *debugMode == true {
			log.Debug("locs: %s", locs)
		}

		reqInJSON, err := request.MakeReq(locs, clientID, i, log)
		if err != nil {
			log.Error("Could not get coordinates in JSON: wrong format")
			continue
		}

		if *debugMode == true {
			log.Info("reqInJSON: %s", *reqInJSON)
		}
		//time.Sleep(1000 * time.Millisecond)

		err = connSend.Send(*destination, "text/json", []byte(*reqInJSON), nil...)
		if err != nil {
			log.Error("Failed to send to server: %v", err)
			return
		}
		i++
	}
}

func recvMessages(subscribed chan bool) {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Error("Cannot connect to server: %v", err.Error())
		return
	}

	if *debugMode == true {
		log.Debug("Subscribing to %s", *queueFormat+clientID)
	}

	sub, err := conn.Subscribe(*queueFormat+clientID, stomp.AckAuto)
	if err != nil {
		log.Error("Cannot subscribe to %s: %v", *queueFormat+clientID, err.Error())
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
			log.Info("Got message: %s", message)
		}
		msgCount++

	}
}

func main() {

	// logger configuration
	defer log.Close()

	flag.Parsed()
	flag.Parse()

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(*login, *passcode),
		stomp.ConnOpt.Host("/"),
	}

	subscribed := make(chan bool)

	if *debugMode == true {
		log.Debug("main")
	}

	go recvMessages(subscribed)
	// wait until we know the receiver has subscribed
	<-subscribed

	go sendMessages()

	<-stop
	<-stop
}
