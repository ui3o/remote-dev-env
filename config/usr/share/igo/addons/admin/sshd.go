package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type UserLoginData struct {
	Cookie string `json:"cookie"`
	Domain string `json:"domain"`
}

func sshdWs(c *gin.Context) {
	if strings.ToLower(c.Request.Header.Get("Connection")) == "upgrade" &&
		strings.ToLower(c.Request.Header.Get("Upgrade")) == "websocket" {
		if envPort := os.Getenv("PORT_HIDDEN_SSHD"); envPort != "" {
			port := envPort
			upgrader := websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool { return true },
			}

			wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				log.Println("WebSocket upgrade error:", err)
				return
			}
			sshConn, err := net.Dial("tcp", "127.0.0.1:"+port)
			if err != nil {
				log.Println("SSH connect error:", err)
				wsConn.WriteMessage(websocket.TextMessage, []byte("SSH server unavailable"))
				return
			}
			defer sshConn.Close()

			log.Println("Tunnel established")

			// WS → TCP
			go func() {
				for {
					_, data, err := wsConn.ReadMessage()
					if err != nil {
						log.Println("WS read error:", err)
						sshConn.Close()
						return
					}
					sshConn.Write(data)
				}
			}()

			// TCP → WS
			buf := make([]byte, 1024)
			for {
				n, err := sshConn.Read(buf)
				if err != nil {
					log.Println("SSH read error:", err)
					break
				}
				wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
		} else {
			log.Println("No port is defined for sshd!!!")
		}
	}
}
