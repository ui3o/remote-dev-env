package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type RuntimeConfig struct {
	Port             int
	TemplateRootPath string
	DomainPath       string
}

func serveWebsocket(remoteUrl string, c *gin.Context) {
	// Handle WebSocket upgrade
	if strings.ToLower(c.Request.Header.Get("Connection")) == "upgrade" &&
		strings.ToLower(c.Request.Header.Get("Upgrade")) == "websocket" {

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}

		if clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil); err != nil {
			log.Println("Client upgrade error:", err)
			return
		} else {

			defer clientConn.Close()

			log.Println("WebSocket proxy connected")
			startListen := func(src *websocket.Conn, errCh chan error) {
				for {
					msgType, msg, err := src.ReadMessage()
					if err != nil {
						errCh <- err
						return
					}
					// todo handle message read here
					err = src.WriteMessage(msgType, msg)
					if err != nil {
						errCh <- err
						return
					}
				}
			}
			// Proxy messages in both directions
			errCh := make(chan error, 2)
			go startListen(clientConn, errCh)
			<-errCh
		}
	}
}
