package main

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	GAME_WIDTH  = 1280
	GAME_HEIGHT = 720
	GAME_FPS    = 60

	RECT_WIDTH = 100 // pixel

	MID_COORD = (GAME_HEIGHT - RECT_WIDTH) / 2
)

type hub struct {
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
}

type Message struct {
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

func (h *hub) getPState() []PlayerState {
	pstate := make([]PlayerState, 0, 2)
	for c := range h.clients {
		pstate = append(pstate, *c.playerState)
	}
	return pstate
}

func (h *hub) updateGameState() error {
	ps := h.getPState()
	msg := Message{
		Action: "UPDATE_FRAME",
		Content: struct {
			PlayersState []PlayerState `json:"players_state"`
		}{
			PlayersState: ps,
		},
	}
	m, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error while marshaling json %v", err)
	}
	go func(m []byte) {
		h.broadcast <- m
	}(m)
	return nil
}

func (h *hub) run() {
	ticker := time.NewTicker(time.Second / GAME_FPS)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if len(h.clients) != 2 {
				continue
			}
			err := h.updateGameState()
			if err != nil {
				continue
			}
		case client := <-h.register:
			coords := []struct {
				x int
				y int
			}{
				{x: 10, y: MID_COORD},
				{x: GAME_WIDTH - 20, y: MID_COORD},
			}
			h.clients[client] = true

			pstate := make([]PlayerState, 0, 2)
			i := 0
			for c := range h.clients {
				if i >= len(coords) {
					break
				}
				c.playerState = &PlayerState{X: coords[i].x, Y: coords[i].y}
				pstate = append(pstate, *c.playerState)
				i++
			}

			if len(h.clients) != 2 {
				continue
			}

			msg := Message{
				Action: "GAME_START",
				Content: struct {
					PlayersState []PlayerState `json:"players_state"`
				}{
					PlayersState: pstate,
				},
			}
			j, err := json.Marshal(msg)
			if err != nil {
				fmt.Printf("error while marshaling json %v", err)
				continue
			}
			go func(m []byte) {
				h.broadcast <- m
			}(j)
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
