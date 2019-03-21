package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "sensor-server:8080", "http service address")

var clientId string
var status int

func WebSocketMessage() string {
	return fmt.Sprintf("{ \"identifier\": \"%s\", \"status\" : %d }", clientId, status)
}

func main() {

	uuid, err := uuid.NewUUID()

	if err == nil {
		clientId = uuid.String()
	} else {
		clientId = "notsetted"
	}

	status = 0

	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/receiver"}
	log.Printf("Connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			} else if messageType == websocket.PingMessage {
				c.WriteMessage(websocket.PongMessage, []byte{})
			} else if len(message) == 0 {
				log.Printf("Connected...")
			} else {
				log.Printf("Event Arrived: %s", message)
			}
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			message := WebSocketMessage()
			log.Println("Sending: ", message)
			err := c.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				log.Println("write:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
