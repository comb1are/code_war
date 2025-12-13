package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	ID   string
	Name string // <-- Храним имя здесь
	room *Room
	conn *websocket.Conn
	send chan WsResponse
}

func serveWs(room *Room, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// 1. Получаем имя из параметров URL: /ws?name=Alex
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "Аноним"
	}

	// Используем ID как случайную строку или IP, но имя храним отдельно
	id := name // Для простоты ID будет равен Имени (в реальном проде нужен уникальный ID + Имя)

	client := &Client{
		ID:   id,
		Name: name,
		room: room,
		conn: conn,
		send: make(chan WsResponse, 256),
	}

	client.room.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.room.unregister <- c
		c.conn.Close()
	}()
	for {
		var msg IncomingMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			break
		}
		c.room.broadcast <- &ClientMessage{client: c, content: msg}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteJSON(message)
		}
	}
}

// Удаляем init(), так как rand нам здесь больше не нужен для генерации ID
