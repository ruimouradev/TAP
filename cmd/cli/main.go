// cmd/cli: the TAP command line client. Sends the raw RFC commands typed by
// the user and prints everything the server sends, in real time. Two goroutines:
// readLoop (socket -> stdout) and writeLoop (stdin -> socket).
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"the-answer-protocol/internal/config"
)

func main() {
	addr := "localhost:" + config.Port()
	if len(os.Args) >= 2 {
		addr = os.Args[1]
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot connect:", err)
		os.Exit(1)
	}
	defer conn.Close()

	done := make(chan struct{})
	go readLoop(conn, done)
	writeLoop(conn, done)
}

// readLoop prints server lines until the connection closes.
func readLoop(conn net.Conn, done chan<- struct{}) {
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
	close(done)
}

// writeLoop sends stdin lines to the server, ending on QUIT or server close.
func writeLoop(conn net.Conn, done <-chan struct{}) {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		select {
		case <-done:
			return
		default:
		}
		line := sc.Text()
		fmt.Fprintf(conn, "%s\n", line)
		if strings.EqualFold(strings.TrimSpace(line), "QUIT") {
			return // closes the connection (defer in main)
		}
	}
}
