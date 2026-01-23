package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

const numChildren = 4 // number of pre-forked child processes

func preforkServer(addr string) {

	// 1. create listening socket
	// listen for incoming TCP connections
	// create tcp socket, bind to addr and start listening
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Server listening on", listener.Addr().String())

	// get raw FD from Listener.
	file, err := listener.(*net.TCPListener).File()
	if err != nil {
		log.Fatal(err)
	}
	listenFd := int(file.Fd())

	log.Printf("[Parent %d] Pre-forking %d children...\n", os.Getpid(), numChildren)

	// 2. spawn child processes
	for i := 0; i < numChildren; i++ {
		ret, _, errPtr := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)

		if errPtr != 0 {
			log.Fatalf("Fork failed: %v", errPtr)
		}

		if ret == 0 {
			// --- child process ---
			runChildWorker(i, listenFd)
			os.Exit(0)
		} else {
			// parent process continues to next fork
			log.Printf("[Parent] Started worker child PID: %d", ret)
		}
	}

	// 3. parent process focuses on monitoring and reaping child processes
	handleParentSignals()
}

func runChildWorker(id int, listenFd int) {
	log.Printf("[Child %d (PID: %d)] Ready to accept connections", id, os.Getpid())

	for {
		// 4. accept connections in child process
		// all child processes share the same listenFd
		// when a connection request arrives, os schedules one of the children to accept it
		nfd, _, err := syscall.Accept(listenFd)
		if err != nil {
			log.Printf("[Child %d] Accept error: %v", id, err)
			continue
		}

		log.Printf("[Child %d] Handling connection (FD: %d)", id, nfd)

		// call the connection handler
		handleRawFD(nfd)

		log.Printf("[Child %d] Finished connection (FD: %d)", id, nfd)
	}
}

func handleParentSignals() {
	// parent process waits for signals to avoid exiting
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGCHLD)

	// after forking children, parent process monitors signals
	for sig := range sigChan {
		switch sig {
		case syscall.SIGCHLD:
			// reap zombie processes
			for {
				pid, err := syscall.Wait4(-1, nil, syscall.WNOHANG, nil)
				if err != nil || pid <= 0 {
					break
				}
				log.Printf("[Parent] Reaped child PID: %d", pid)
			}
		case syscall.SIGINT, syscall.SIGTERM:
			log.Println("[Parent] Terminating...")
			os.Exit(0)
		}
	}
}
