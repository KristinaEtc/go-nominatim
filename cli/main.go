package main

import (
	"database/sql"
	"flag"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib"
	//"fmt"
	"bufio"
	"log"
	"os"
	"strconv"

	_ "github.com/lib/pq"
	// "testing"
	//"fmt"
	"strings"
)

var configFile = "config.json"

var testFile = "test.csvt"
var numOfReq = 0

type ConfigDB struct {
	DBname, Host, User, Password string
}

type LocationParams struct {
	lat  float64
	lon  float64
	zoom int
}

type Params struct {
	locParams      LocationParams
	format         string
	addressDetails bool
	sqlOpenStr     string
	config         ConfigDB
	db             *sql.DB
	speedTest      bool
}

/*-------------------------
  Config option structures
-------------------------*/

var uuid string

type QueueOptConf struct {
	QueueName      string
	QueuePriorName string
	ResentFullReq  bool
}

type DiagnosticsConf struct {
	CoeffEMA      float64
	TopicName     string
	TimeOut       int // in seconds
	MachineID     string
	CoeffSeverity float64
}

type ConnectionConf struct {
	ServerAddr     string
	ServerUser     string
	ServerPassword string
	QueueFormat    string
	HeartBeatError int
	HeartBeat      int
}

// NominatimConf options
type NominatimConf struct {
	User     string
	Password string
	Host     string
	DBname   string
}

// ConfFile is a file with all program options
type ConfFile struct {
	Name        string
	DirWithUUID string
	ConnConf    ConnectionConf
	DiagnConf   DiagnosticsConf
	QueueConf   QueueOptConf
	NominatimDB NominatimConf
}

var globalOpt = ConfFile{
	Name:        "name",
	DirWithUUID: ".go-stomp-nominatim/",

	ConnConf: ConnectionConf{
		ServerAddr:     "localhost:61615",
		QueueFormat:    "/queue/",
		ServerUser:     "",
		ServerPassword: "",
		HeartBeat:      30,
		HeartBeatError: 15,
	},
	QueueConf: QueueOptConf{
		ResentFullReq:  true,
		QueueName:      "/queue/nominatimRequest",
		QueuePriorName: "/queue/nominatimPriorRequest",
	},
	DiagnConf: DiagnosticsConf{
		CoeffEMA:      0.1,
		TopicName:     "/topic/worker.status",
		TimeOut:       5,
		MachineID:     "defaultName",
		CoeffSeverity: 2,
	},
	NominatimDB: NominatimConf{
		DBname:   "nominatim",
		Host:     "localhost",
		User:     "geocode1",
		Password: "_geocode1#",
	},
}

//--------------------------------------------------------------------------

func (p *Params) getParams() {

	flag.StringVar(&(p.format), "f", "json", "format")
	flag.Float64Var(&p.locParams.lat, "a", 53.902238, "lat")
	flag.Float64Var(&p.locParams.lon, "b", 27.561916, "log")
	flag.IntVar(&p.locParams.zoom, "z", 18, "zoom")
	flag.BoolVar(&p.addressDetails, "d", false, "addressDetails")
	flag.BoolVar(&p.speedTest, "t", false, "speed test")
	flag.StringVar(&(testFile), "n", testFile, "name of your test file")

	flag.Parse()
	//flag.Parsed()

	return
}

/*func (p *Params) configurateDB() {

	file, err := os.Open(configFile)
	if err != nil {
		log.Printf("No configurate file")
	} else {
		defer file.Close()
		decoder := json.NewDecoder(file)
		configuration := ConfigDB{}
		err := decoder.Decode(&configuration)
		if err != nil {
			log.Println("error: ", err)
		}
		p.config = configuration
	}
}*/

func initReverseGeocode() (*Nominatim.ReverseGeocode, error) {

	log.Println("initReverseGeocode")

	sqlOpenStr := "dbname=" + globalOpt.NominatimDB.DBname +
		" host=" + globalOpt.NominatimDB.Host +
		" user=" + globalOpt.NominatimDB.User +
		" password=" + globalOpt.NominatimDB.Password

	log.Printf("sqlOpenStr=%s", sqlOpenStr)

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		return nil, err
	}
	return reverseGeocode, nil
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	p := Params{}

	//p.configurateDB()
	p.getParams()

	config.ReadGlobalConfig(&globalOpt, "go-stomp-nominatim options")

	//log.Print(p)
	var l []LocationParams

	if p.speedTest == false {
		numOfReq = 1
		l = append(l, p.locParams)
	} else {
		file, err := os.Open(testFile)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		defer file.Close()

		reader := bufio.NewReader(file)
		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			locs := scanner.Text()

			locSlice := strings.Split(locs, ",")
			locStr := LocationParams{}
			locStr.lat, err = strconv.ParseFloat(locSlice[0], 32)
			if err != nil {
				log.Print(err)
				continue
			}
			//log.Println(locStr.lat)
			locStr.lon, err = strconv.ParseFloat(locSlice[1], 32)
			if err != nil {
				log.Print(err)
				continue
			}
			//log.Println(locStr.lon)
			locStr.zoom, err = strconv.Atoi(locSlice[2])
			if err != nil {
				log.Print(err)
				continue
			}
			//log.Println(locStr.zoom)
			l = append(l, locStr)
		}
		for _, data := range l {
			log.Printf("%f %f %d\n", data.lat, data.lon, data.zoom)
			numOfReq++
		}
	}

	reverseGeocode, err := initReverseGeocode()
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
	defer reverseGeocode.Close()

	found := 0
	errors := 0

	for _, data := range l {
		//log.Println("Request:", i)

		//oReverseGeocode.SetLanguagePreference()
		reverseGeocode.SetIncludeAddressDetails(p.addressDetails)
		reverseGeocode.SetZoom(data.zoom)
		reverseGeocode.SetLocation(data.lat, data.lon)

		place, err := reverseGeocode.Lookup(globalOpt.QueueConf.ResentFullReq)

		if err != nil {
			//log.Println(err)
			errors++
			continue
		}
		found++
		log.Println(place)
	}
	log.Println("found:", found, "errors:", errors)

}
