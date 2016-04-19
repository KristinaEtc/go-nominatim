package main

import (
	"github.com/bitly/go-nsq"
	//"go-nsq"
	"log"
	"os"
	"strconv"
)

var (
	addr           string
	topicToPublish string
)

func main() {

	config := nsq.NewConfig()

	w, errNewPr := nsq.NewProducer(addr, config)
	if errNewPr != nil {
		log.Println("Couldn't create new producer")
		panic(errNewPr.Error())
	}

	var pMsg string = "53.9009279,27.5592055,18"
	errPubl := w.Publish(p.topicToPublish, []byte(pMsg))
	if errPubl != nil {
		log.Println("Couldn't connect")
	}
	w.Stop()
}
