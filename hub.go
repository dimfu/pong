package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	GAME_WIDTH  = 1280
	GAME_HEIGHT = 720
	GAME_FPS    = 60

	RECT_WIDTH  = 100 // pixel
	RECT_HEIGHT = 10

	MID_COORD = (GAME_HEIGHT - RECT_WIDTH) / 2

	// ball
	MAX_ANGLE = 35
)

type hub struct {
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
	Ball       *BallState
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
		Ball: &BallState{
			X: GAME_WIDTH / 2.0, Y: GAME_HEIGHT / 2.0, Radius: 15.0, Angle: 0.0,
			SpeedX: 0.0, SpeedY: 0.0, Velocity: 10,
		},
	}
}

func (h *hub) getPState() []PlayerState {
	pstate := make([]PlayerState, 0, 2)
	for c := range h.clients {
		pstate = append(pstate, *c.playerState)
	}
	return pstate
}

func (h *hub) restartBall() {
	h.Ball.X = GAME_WIDTH / 2.0
	h.Ball.Y = GAME_HEIGHT / 2.0

	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)

	angleDegrees := rng.Intn(2*MAX_ANGLE+1) - MAX_ANGLE
	h.Ball.Angle = float64(angleDegrees) * (math.Pi / 180.0)
	h.Ball.SpeedX = h.Ball.Velocity * math.Cos(h.Ball.Angle)
	h.Ball.SpeedY = h.Ball.Velocity * math.Sin(h.Ball.Angle)
	if rng.Intn(2) == 0 {
		h.Ball.SpeedX = h.Ball.Velocity
	} else {
		h.Ball.SpeedX = -h.Ball.Velocity
	}
}

func (h *hub) updateBallPos() {
	ps := h.getPState()

	h.Ball.X += h.Ball.SpeedX
	h.Ball.Y += h.Ball.SpeedY

	if h.Ball.Y-h.Ball.Radius <= 0 {
		h.Ball.Y = h.Ball.Radius
		h.Ball.SpeedY = -h.Ball.SpeedY
	}

	if h.Ball.Y+h.Ball.Radius >= GAME_HEIGHT {
		h.Ball.Y = GAME_HEIGHT - h.Ball.Radius
		h.Ball.SpeedY = -h.Ball.SpeedY
	}

	if h.Ball.X+h.Ball.Radius > float64(ps[0].X) &&
		h.Ball.X-h.Ball.Radius < float64(ps[0].X+RECT_HEIGHT+10) &&
		h.Ball.Y+h.Ball.Radius > float64(ps[0].Y) &&
		h.Ball.Y-h.Ball.Radius < float64(ps[0].Y+RECT_WIDTH) {
		h.Ball.SpeedX = -h.Ball.SpeedX
	}

	if h.Ball.X+h.Ball.Radius > float64(ps[1].X) &&
		h.Ball.X-h.Ball.Radius < float64(ps[1].X+RECT_HEIGHT) &&
		h.Ball.Y+h.Ball.Radius > float64(ps[1].Y) &&
		h.Ball.Y-h.Ball.Radius < float64(ps[1].Y+RECT_WIDTH) {
		h.Ball.SpeedX = -h.Ball.SpeedX
	}

	if h.Ball.X-h.Ball.Radius <= 0 {
		h.restartBall()
	}

	if h.Ball.X+h.Ball.Radius >= GAME_WIDTH {
		h.restartBall()
	}
}

func (h *hub) updateGameState() error {
	ps := h.getPState()

	h.updateBallPos()

	msg := Message{
		Action: "UPDATE_FRAME",
		Content: struct {
			PlayersState []PlayerState `json:"players_state"`
			BallState    BallState     `json:"ball_state"`
		}{
			PlayersState: ps,
			BallState:    *h.Ball,
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

			h.restartBall()

			msg := Message{
				Action: "GAME_START",
				Content: struct {
					PlayersState []PlayerState `json:"players_state"`
					BallState    BallState     `json:"ball_state"`
				}{
					PlayersState: pstate,
					BallState:    *h.Ball,
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
