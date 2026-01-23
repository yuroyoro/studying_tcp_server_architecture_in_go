package main

import (
	"log"
	"net"
)

func microthreadServer(addr string) {
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

		// handle connection in a new goroutine
		go handleConnection(conn)

		log.Println("Finished handling connection from", conn.RemoteAddr().String())
	}
}
