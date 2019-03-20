package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  maxMessageSize,
	WriteBufferSize: maxMessageSize,
}

func checkOrigin(r *http.Request) bool {
	return true
}

func initWs(w http.ResponseWriter, r *http.Request) (c *client, err error) {
	hub := getSocketHubInstance()

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return nil, errors.New("Method not allowed")
	}
	upgrader.CheckOrigin = checkOrigin
	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println(err)
		return nil, errors.New("Method not allowed")
	}

	c = &client{
		send: make(chan []byte, maxMessageSize),
		ws:   ws,
	}

	hub.register <- c

	return c, nil
}

func receiveEventWs(w http.ResponseWriter, r *http.Request) {
	c, err := initWs(w, r)

	if err != nil {
		return
	}

	go c.writeChannel()
	c.readReceiver()
}

func main() {

	hub := getSocketHubInstance()
	redis := getRedisRepositoryInstance()
	redis.addr = "192.168.99.100:6379"
	redis.password = "redispass"
	initRedisRepository(redis)
	go redis.redisSetRun()
	go hub.run()
	http.HandleFunc("/receive", receiveEventWs)
	http.ListenAndServe(":8080", nil)
}
