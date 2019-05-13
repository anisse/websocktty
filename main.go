package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh/terminal"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	signal.Notify(interrupt, syscall.SIGTERM)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/"}
	log.Printf("connecting to %s. `kill -15 %d` to stop", u.String(), os.Getpid())

	proto := make(http.Header)
	proto.Add("Sec-WebSocket-Protocol", "binary")
	c, _, err := websocket.DefaultDialer.Dial(u.String(), proto)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	oldState, err := terminal.MakeRaw(0)
	if err != nil {
		fmt.Println("Cannot set input terminal in raw mode")
	}
	defer terminal.Restore(0, oldState)

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			fmt.Fprintf(os.Stdout, "%s", message)
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		default:
			buf := make([]byte, 128)
			n, err := os.Stdin.Read(buf)
			err = c.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				log.Println("write:", err)
				return
			}
		}

	}
}
