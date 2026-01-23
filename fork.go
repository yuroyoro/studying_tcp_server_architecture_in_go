package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
)

func forkServer(addr string) {
	// listen for incoming TCP connections
	// create tcp socket, bind to addr and start listening
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Println("Server listening on", listener.Addr().String())

	// get raw FD from Listener.
	file, err := listener.(*net.TCPListener).File()
	if err != nil {
		log.Fatal(err)
	}
	listenFd := int(file.Fd()) //

	log.Printf("Server listening on %s (PID: %d, ListenFD: %d)\n", addr, os.Getpid(), listenFd)

	for {
		// 1. call accept syscall directly
		//   nfd: new socket FD for the accepted connection
		//   sa: socket address of the connected client
		nfd, _, err := syscall.Accept(listenFd)
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		log.Printf("Accepted connection (Client FD: %d)\n", nfd)

		// 2. fork a new process to handle the connection
		//
		// call fork syscall to create a new process for handling the connection
		// if parent process, ret is child's PID.
		// otherwise if child process, ret is 0.
		ret, _, errPtr := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
		if errPtr != 0 {
			log.Printf("Fork failed: %v", errPtr)
			syscall.Close(nfd)
			continue
		}

		if ret == 0 {
			log.Printf("[Child] Forked child process (PID: %d) to handle connection. connection fd = %d", os.Getpid(), nfd)

			// -------- child process ---------
			// child process holds the parent's file descriptors
			// so we need to close the listener in the child process
			syscall.Close(listenFd)
			log.Printf("[Child] Closed listener")

			// call the connection handler
			handleRawFD(nfd)

			log.Printf("[Child %d] Closing connection and exiting.\n", os.Getpid())
			// after finishing the processing, terminate the child process (important!)
			os.Exit(0)
		} else {
			// -------- parent process ---------
			log.Printf("[Parent] Spawned child process (PID: %d) to handle connection. connection fd = %d", ret, nfd)

			// close the connection object in the parent process.
			// (the connection remains alive in the child process)
			syscall.Close(nfd)

			// Note:
			// in a real-world application,
			// it is necessary to call wait4 syscall to avoid zombie processes.
		}
	}
}

func handleRawFD(fd int) {
	defer syscall.Close(fd)

	buf := make([]byte, 1024)
	for {
		// syscall read  (blocking)
		n, err := syscall.Read(fd, buf)
		if err != nil {
			log.Printf("Read error on FD %d: %v", fd, err)
			return
		}
		if n == 0 { // EOF (client closed connection)
			log.Printf("FD %d received EOF", fd)
			return
		}

		fmt.Printf("[Child %d] Received: %s", os.Getpid(), string(buf[:n]))

		// syscall write to echo back
		_, err = syscall.Write(fd, buf[:n])
		if err != nil {
			log.Printf("Write error on FD %d: %v", fd, err)
			return
		}
	}
}
