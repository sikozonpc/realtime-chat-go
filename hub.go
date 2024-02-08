package main

import (
	"bytes"
	"html/template"
	"log"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	sync.RWMutex

	clients map[*Client]bool

	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client

	messages []*Message
}

func NewHub() *Hub {
	return &Hub{
		clients:    map[*Client]bool{},
		broadcast:  make(chan *Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

/*
The select statement inside the loop is used to attempt a non-blocking send operation. If the client's send channel is ready to receive the message (i.e., it's not blocked), case client.send <- msg: will execute, sending the message to the client.

If the client's send channel is not ready to receive the message (i.e., it's blocked because the client is not ready to receive data or the channel is full), the default: case will execute. This will close the client's send channel and remove the client from the h.clients map, effectively disconnecting the client.

This code is a common pattern in Go for handling multiple clients and ensuring that if one client is slow or unresponsive, it doesn't block the entire system from sending messages to other clients.
*/
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.Lock()
			h.clients[client] = true
			h.Unlock()

			log.Printf("client registered %s", client.id)

			for _, msg := range h.messages {
				client.send <- getMessageTemplate(msg)
			}
		case client := <-h.unregister:
			h.Lock()
			if _, ok := h.clients[client]; ok {
				close(client.send)
				log.Printf("client unregistered %s", client.id)
				delete(h.clients, client)
			}
			h.Unlock()
		case msg := <-h.broadcast:
			h.RLock()
			h.messages = append(h.messages, msg)

			for client := range h.clients {
				select {
				case client.send <- getMessageTemplate(msg):
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.RUnlock()
		}
	}
}

func getMessageTemplate(msg *Message) []byte {
	tmpl, err := template.ParseFiles("templates/message.html")
	if err != nil {
		log.Fatalf("template parsing: %s", err)
	}

	// Render the template with the message as data.
	var renderedMessage bytes.Buffer
	err = tmpl.Execute(&renderedMessage, msg)
	if err != nil {
		log.Fatalf("template execution: %s", err)
	}

	return renderedMessage.Bytes()
}
