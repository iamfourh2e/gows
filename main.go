package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type ChatGroup struct {
	clients map[*Client]bool
	mu      sync.Mutex
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	chatGroup = ChatGroup{
		clients: make(map[*Client]bool),
	}
)

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte),
	}

	chatGroup.mu.Lock()
	chatGroup.clients[client] = true
	chatGroup.mu.Unlock()

	defer func() {
		chatGroup.mu.Lock()
		delete(chatGroup.clients, client)
		chatGroup.mu.Unlock()
	}()

	go client.writePump()
	client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// Broadcast message to all clients in the chat group
		chatGroup.mu.Lock()
		for client := range chatGroup.clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(chatGroup.clients, client)
			}
		}
		chatGroup.mu.Unlock()
	}
}

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return
			}
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("Error writing message:", err)
				return
			}
		}
	}
}

func main() {
	r := gin.Default()

	r.GET("/ws", handleWebSocket)
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

	select {}
}
