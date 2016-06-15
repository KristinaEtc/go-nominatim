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

//var log l4g.Logger = l4g.NewLogger()

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

const LogDir = "/home/k/work/go/src/Nominatim/worker/stomp/logs/"

const (
	errorFilename = "error.log"
	infoFilename  = "info.log"
	debugFilename = "debug.log"
)

var (
	bhDebug, bhInfo, bhError, bhConsole     *basic.Handler
	logfileInfo, logfileDebug, logfileError *os.File
	lf                                      slog.LogFactory

	log slf.StructuredLogger
)

func initLoggers() {

	if *consoleMode == true {
		lvl := getLogLevel(*logLevel)
		bhConsole = basic.New(lvl)
		bhConsole.SetWriter(os.Stdout)
	}

	bhDebug = basic.New(slf.LevelDebug)
	bhInfo = basic.New()
	bhError = basic.New(slf.LevelError)

	// optionally define the format (this here is the default one)
	bhInfo.SetTemplate("{{.Time}} [\033[{{.Color}}m{{.Level}}\033[0m] {{.Context}}{{if .Caller}} ({{.Caller}}){{end}}: {{.Message}}{{if .Error}} (\033[31merror: {{.Error}}\033[0m){{end}} {{.Fields}}")

	// TODO: create directory in /var/log, if in linux:
	// if runtime.GOOS == "linux" {
	os.Mkdir("."+string(filepath.Separator)+LogDir, 0766)

	// interestings with err: if not initialize err before,
	// how can i use global logfileInfo?
	var err error
	logfileInfo, err = os.OpenFile(LogDir+infoFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf("Could not open/create %s logfile", LogDir+infoFilename)
	}

	logfileDebug, err = os.OpenFile(LogDir+debugFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf("Could not open/create logfile", LogDir+debugFilename)
	}

	logfileError, err = os.OpenFile(LogDir+errorFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf("Could not open/create logfile", LogDir+errorFilename)
	}

	bhDebug.SetWriter(logfileDebug)
	bhInfo.SetWriter(logfileInfo)
	bhError.SetWriter(logfileError)

	lf = slog.New()
	lf.SetLevel(slf.LevelDebug) //lf.SetLevel(slf.LevelDebug, "app.package1", "app.package2")

	if *consoleMode == true {
		lf.SetEntryHandlers(bhInfo, bhError, bhDebug, bhConsole)
	} else {
		lf.SetEntryHandlers(bhInfo, bhError, bhDebug)
	}

	// make this into the one used by all the libraries
	slf.Set(lf)

	log = slf.WithContext("main-client.go")
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

	flag.Parsed()
	flag.Parse()

	initLoggers()
	defer logfileInfo.Close()
	defer logfileDebug.Close()
	defer logfileError.Close()

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

func getLogLevel(lvl string) slf.Level {

	switch lvl {
	case slf.LevelDebug.String():
		return slf.LevelDebug

	case slf.LevelInfo.String():
		return slf.LevelInfo

	case slf.LevelWarn.String():
		return slf.LevelWarn

	case slf.LevelError.String():
		return slf.LevelError

	case slf.LevelFatal.String():
		return slf.LevelFatal
	case slf.LevelPanic.String():
		return slf.LevelPanic
	default:
		return slf.LevelDebug
	}
}
