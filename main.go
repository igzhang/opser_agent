package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func ExecuteCmd(cmd string) (string, error) {
	output, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Println("Command execution failed:", err)
		return "", err
	}

	return string(output), nil
}

func main() {
	serverAddr := os.Getenv("host")
	if len(serverAddr) == 0 {
		log.Fatalln("Env host not found!")
	}

	// signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	done := make(chan struct{})

	// connect websocket server
	c, _, err := websocket.DefaultDialer.Dial(serverAddr, nil)
	if err != nil {
		log.Fatalf("WebSocket dial error: %v", err)
	}
	defer c.Close()

	// get hostname as uuid
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %v", err)
	}
	log.Printf("My Hostname: %s\n", hostname)

	// receve handler
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}
			log.Printf("Received message from server: %s\n", message)
			commandStdout, err := ExecuteCmd(string(message))

			err_msg := ""
			if err != nil {
				err_msg = err.Error()
			}
			c.WriteJSON(map[string]string{
				"out": commandStdout,
				"err": err_msg,
			})
		}
	}()

	// ping message and ctrl+c
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			err := c.WriteMessage(websocket.PingMessage, []byte(hostname))
			if err != nil {
				log.Fatalf("Write error: %v", err)
			}
		case <-interrupt:
			log.Println("Interrupt signal received, closing connection...")
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("Write close error:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
