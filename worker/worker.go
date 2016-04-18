package main

import (
	"github.com/bitly/go-nsq"
	"log"
	"sync"
)

const (
	HOST               string = "localhost"
	PORT               string = ":4150"
	topicToSubscribe          = "topicName"
	channelToSubscribe        = "channelName"
)

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	config := nsq.NewConfig()
	consumerPointer, err := nsq.NewConsumer(topicToSubscribe, channelToSubscribe, config)
	if err != nil {
		log.Panic(err)
		return
	}
	consumerPointer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		log.Printf("Got a message: %v", message)
		wg.Done()
		return nil
	}))

	err = consumerPointer.ConnectToNSQD(HOST + PORT)
	if err != nil {
		log.Panic("Could not connect")
	}
	wg.Wait()

}
