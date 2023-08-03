package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

var (
	host string
	port int
)

func init() {
	flag.StringVar(&host, "h", "", "remote host")
	flag.IntVar(&port, "p", 0, "remote port")
}

func main() {
	flag.Parse()

	if host == "" {
		fmt.Println("Host is required, use -host or -h to specify.")
		os.Exit(1)
	}

	if port < 1025 || port > 65535 {
		fmt.Println("Port is required between 1025 and 65534, use -port or -p to specify.")
		os.Exit(1)
	}

	conn, err := net.DialTimeout("tcp", host+":"+strconv.Itoa(port), 5*time.Second)
	if err != nil {
		fmt.Println("Error connecting to the server:", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	conn.Write([]byte("PING"))

	buf := make([]byte, 1024)
	reqLen, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading response:", err.Error())
		os.Exit(1)
	}

	response := string(buf[:reqLen])
	if response == "OK" {
		fmt.Println("Success.")
	} else {
		fmt.Println("Unexpected response:", response)
	}
}
