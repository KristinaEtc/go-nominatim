package main

import (
	"Nominatim/lib/utils/basic"
	"Nominatim/lib/utils/fileproc"
	"Nominatim/lib/utils/request"
	"flag"
	"github.com/go-stomp/stomp"
	"github.com/ventu-io/slf"
	"github.com/ventu-io/slog"
	"os"
	"path/filepath"
)

const (
	defaultPort = ":61613"
	clientID    = "clientID"
)

var (
	serverAddr  = flag.String("server", "localhost:61614", "STOMP server endpoint")
	destination = flag.String("topic", "/queue/nominatimRequest", "Destination topic")
	queueFormat = flag.String("queue", "/queue/", "Queue format")
	login       = flag.String("login", "client1", "Login for authorization")
	passcode    = flag.String("pwd", "111", "Passcode for authorization")
	testFile    = flag.String("testfile", "../test.csv", "testfile with coordinates")

	logLevel    = flag.String("loglevel", "INFO", "IFOO, DEBUG, ERROR, WARN, PANIC, FATAL")
	consoleMode = flag.Bool("console", true, "Console output")

	stop = make(chan bool)
)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("guest", "guest"),
	stomp.ConnOpt.Host("/"),
}

func sendMessages() {
	defer func() {
		stop <- true
	}()

	_, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return
	}

	connSend, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %v", err.Error())
		return
	}

	fs, err := fileproc.NewFileScanner(*testFile)
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

		err = connSend.Send(*destination, "text/json", []byte(*reqInJSON), nil...)
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

	conn, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Errorf("Cannot connect to server: %v", err.Error())
		return
	}

	log.Debugf("Subscribing to %s", *queueFormat+clientID)

	sub, err := conn.Subscribe(*queueFormat+clientID, stomp.AckAuto)
	if err != nil {
		log.Errorf("Cannot subscribe to %s: %v", *queueFormat+clientID, err.Error())
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

	flag.Parse()
	slflog.InitLoggers(*logPath, *logLevel)
	log := slf.WithContext("stomp-client.go")

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(*login, *passcode),
		stomp.ConnOpt.Host("/"),
	}

	subscribed := make(chan bool)
	log.Info("starting working...")

	go recvMessages(subscribed)
	// wait until we know the receiver has subscribed
	<-subscribed

	go sendMessages()

	<-stop
	<-stop
}
