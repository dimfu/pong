package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type PlayerState struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type client struct {
	hub         *hub
	conn        *websocket.Conn
	send        chan []byte
	playerState *PlayerState
}

func (c *client) read() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		message = bytes.TrimSpace(bytes.Replace(message, []byte{'\n'}, []byte{' '}, -1))
		var payload Message
		err = json.Unmarshal(message, &payload)
		if err != nil {
			log.Printf("error unmarshaling message: %v", err)
			break
		}

		switch payload.Action {
		case "MOVE_UP":
			if c.playerState.Y-1 >= 0 {
				c.playerState.Y -= 1
			}
		case "MOVE_DOWN":
			if (c.playerState.Y+RECT_WIDTH)+1 <= GAME_HEIGHT {
				c.playerState.Y += 1
			}
		}

		c.hub.broadcast <- message
	}
}

func (c *client) write() {
	ticker := time.NewTicker(60 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *client) sendmsg(msg Message) error {
	j, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error while marshaling json %v", err)
	}
	c.hub.broadcast <- j
	return nil
}

func servews(hub *hub, w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatalf("error while upgrading ws connection %v", err)
	}

	client := &client{hub: hub, conn: c, send: make(chan []byte, 256)}
	client.hub.register <- client

	go client.write()
	go client.read()
}
