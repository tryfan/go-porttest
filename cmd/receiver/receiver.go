package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
)

var (
	port    int
	timeout int
)

const (
	CONN_TYPE = "tcp"
)

func init() {
	flag.IntVar(&port, "p", 0, "port to listen on")
	flag.IntVar(&timeout, "t", 10, "program timeout in seconds, default 10")
}

func main() {
	flag.Parse()

	if port < 1025 || port > 65535 {
		fmt.Println("Port is required between 1025 and 65534, use -port or -p to specify.")
		os.Exit(1)
	}

	// Check if the process is already running as a background process.
	if os.Getppid() != 1 {
		// Create a new background process.
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Start()
		fmt.Println("Daemon process started with PID:", cmd.Process.Pid)
		return
	}

	ipAddress := getSystemIP()

	// Listen for incoming connections.
	l, err := net.Listen(CONN_TYPE, ipAddress+":"+strconv.Itoa(port))
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer l.Close()

	// Exit in 30 seconds
	time.AfterFunc(time.Second*10, func() {
		fmt.Println("Quitting after 10 seconds")
		os.Exit(0)
	})

	fmt.Println("Listening on " + ipAddress + ":" + strconv.Itoa(port))
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn) {
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	// Read the incoming connection into the buffer.
	reqLen, err := conn.Read(buf)
	_ = reqLen
	if err != nil {
		fmt.Println("Error reading:", err.Error())
	}
	// Send a response back to person contacting us.
	conn.Write([]byte("OK"))
	// Close the connection when you're done with it.
	conn.Close()
}

func getSystemIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println("Error getting IP address:", err.Error())
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}

	return "127.0.0.1"
}
