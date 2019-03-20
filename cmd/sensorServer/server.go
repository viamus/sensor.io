package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ReceiverStatus int

const (
	Receiver_Enabled  ReceiverStatus = 1
	Receiver_Disabled ReceiverStatus = 0
)

type receiver struct {
	Id     string          `json:"identifier"`
	Status *ReceiverStatus `json:"status"`
}

type client struct {
	ws       *websocket.Conn
	send     chan []byte
	receiver *receiver
}

type serverHub struct {
	listeners  map[*client]bool
	broadcast  chan string
	register   chan *client
	unregister chan *client

	content string
}

var serverHubInstance *serverHub
var serverHubOnce sync.Once

func (r ReceiverStatus) receiverStatusToString() string {

	switch r {
	case Receiver_Enabled:
		return "Enabled"
	case Receiver_Disabled:
		return "Disabled"
	default:
		return "Unknown"
	}
}

func getSocketHubInstance() *serverHub {
	serverHubOnce.Do(func() {
		serverHubInstance = &serverHub{
			listeners:  make(map[*client]bool),
			register:   make(chan *client),
			unregister: make(chan *client),
			broadcast:  make(chan string),
		}
	})

	return serverHubInstance
}

func (h *serverHub) run() {
	log.Println("Starting WebSocket Routine ...")
	i := 0
	for {
		select {
		case c := <-h.register:
			i = i + 1
			log.Println("Registering Listener ", i)
			h.listeners[c] = true
			c.send <- []byte(h.content)
			break

		case c := <-h.unregister:
			i = i - 1
			log.Println("Unregistering Listener ", i)
			_, ok := h.listeners[c]
			if ok {
				delete(h.listeners, c)
				close(c.send)
			}
			break

		case m := <-h.broadcast:
			log.Println("Broadcasting Listener ")
			h.content = m
			h.broadcastMessage()
			break
		}
	}
}

func (h *serverHub) broadcastMessage() {
	for c := range h.listeners {
		select {
		case c.send <- []byte(h.content):
			break
		default:
			close(c.send)
			delete(h.listeners, c)
		}
	}
}

func (c *client) readReceiver() {
	h := getSocketHubInstance()
	r := getRedisRepositoryInstance()

	defer func() {
		h.unregister <- c
		c.ws.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, message, err := c.ws.ReadMessage()

		if err != nil {
			break
		}

		if messageType != websocket.PongMessage {
			identifier := &receiver{}
			err = json.Unmarshal([]byte(message), identifier)

			if err != nil {
				log.Fatal(err)
				break
			}

			c.receiver = identifier

			r.redisSet <- &redisSet{key: c.receiver.Id, value: c.receiver.Status.receiverStatusToString(), expirationTime: 0}
		}
	}
}

func (c *client) writeChannel() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (c *client) write(mt int, message []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, message)
}
