package main

import (
	"io"
	"log"
	"net"
)

// sample TCP echo server implementation
func simpleServer(addr string) {

	// listen for incoming TCP connections
	// create tcp socket, bind to addr and start listening
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Println("Server listening on", listener.Addr().String())

	for {
		// accept incoming connections
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}

		log.Println("Accepted connection from", conn.RemoteAddr().String())

		// handle connection
		handleConnection(conn)

		log.Println("Finished handling connection from", conn.RemoteAddr().String())
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		// read data from connection
		n, err := conn.Read(buf)

		// if client closed the connection, exit the loop
		if err == io.EOF {
			log.Println("Connection closed by client")
			return
		}
		// other errors
		if err != nil {
			log.Println("Error reading from connection:", err)
			return
		}
		log.Printf("Received %d bytes: %s", n, string(buf[:n]))

		// echo back the received data
		_, err = conn.Write(buf[:n])
		if err != nil {
			log.Println("Error writing to connection:", err)
			return
		}
	}
}
