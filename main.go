package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

var CmdNothingID uint = 0

type Cmd struct {
	ID      uint   `json:"id"`
	Context string `json:"context"`
}

type CmdResult struct {
	ID        uint   `json:"id"`
	IsSuccess bool   `json:"is_success"`
	Context   string `json:"stdout"`
	Hostname  string `json:"hostname"`
}

func ExecuteCmd(cmd string) (string, bool) {
	command := exec.Command("bash", "-c", cmd)

	stdout, err := command.StdoutPipe()
	if err != nil {
		return err.Error(), false
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return err.Error(), false
	}

	if err := command.Start(); err != nil {
		return err.Error(), false
	}

	stderr_string, _ := io.ReadAll(stderr)
	log.Printf("exec stderr %s\n", stderr_string)

	stdout_string, _ := io.ReadAll(stdout)
	log.Printf("exec stdout %s\n", stdout_string)

	if err := command.Wait(); err != nil {
		return string(stderr_string), false
	}

	return string(stdout_string), true
}

func main() {
	serverAddr := os.Getenv("server")
	if len(serverAddr) == 0 {
		log.Fatalln("Env server not found!")
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

	// command handler
	go func() {
		defer close(done)
		for {
			// ask for job
			c.WriteJSON(CmdResult{
				ID:       CmdNothingID,
				Hostname: hostname,
			})

			var cmd Cmd
			if err := c.ReadJSON(&cmd); err != nil {
				log.Println("Read error:", err)
				return
			}

			// nothing to do
			if cmd.ID == CmdNothingID {
				time.Sleep(3 * time.Second)
				continue
			}
			log.Printf("Received message from server: %d %s\n", cmd.ID, cmd.Context)

			commandStdout, IsSuccess := ExecuteCmd(cmd.Context)

			c.WriteJSON(CmdResult{
				ID:        cmd.ID,
				IsSuccess: IsSuccess,
				Context:   commandStdout,
				Hostname:  hostname,
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
				log.Fatalf("Failed to send heartbeat: %v", err)
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
