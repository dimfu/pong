package main

import (
	"fmt"
	"log"
)

type hub struct {
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
}

type message struct {
	Action  string      `json:"action"`
	Content interface{} `json:"content"`
}

func newhub() *hub {
	return &hub{
		clients:    make(map[*client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *client),
		unregister: make(chan *client),
	}
}

func (h *hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			for c, exists := range h.clients {
				if exists {
					msg := message{
						Action: "PLAYER_IN",
						Content: struct {
							Message     string `json:"message"`
							Connections int    `json:"connections"`
						}{
							Message:     fmt.Sprintf("new connections from %v", &c),
							Connections: len(h.clients),
						},
					}
					err := c.sendmsg(msg)
					if err != nil {
						log.Print(err.Error())
						continue
					}
				}
			}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)

				for c, exists := range h.clients {
					if exists {
						msg := message{
							Action: "PLAYER_OUT",
							Content: struct {
								Message     string `json:"message"`
								Connections int    `json:"connections"`
							}{
								Message:     fmt.Sprintf("client %v closed the connection", &c),
								Connections: len(h.clients),
							},
						}

						err := c.sendmsg(msg)
						if err != nil {
							log.Print(err.Error())
							continue
						}
					}
				}
			}
		}
	}
}
