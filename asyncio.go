package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
)

const MaxEvents = 64

func asyncioServer(addr string) {
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

	// set socket to non-blocking mode
	syscall.SetNonblock(listenFd, true)

	// create a epoll instance
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatal("EpollCreate1:", err)
	}
	defer syscall.Close(epfd)

	// set listen socket fd to epoll monitoring
	// EPOLLIN: watch for read availability
	event := syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(listenFd),
	}
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, listenFd, &event); err != nil {
		log.Fatal("EpollCtl (listenFd):", err)
	}

	log.Printf("Epoll Server started on %s (PID: %d)\n", addr, os.Getpid())

	events := make([]syscall.EpollEvent, MaxEvents)

	// event loop
	for {
		// wait for io events (blocking)
		n, err := syscall.EpollWait(epfd, events, -1)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			log.Fatal("EpollWait:", err)
		}

		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)

			// check if the event is for the listening socket
			if fd == listenFd {
				// --- New connection request ---
				nfd, _, err := syscall.Accept(fd)
				if err != nil {
					log.Println("Accept error:", err)
					continue
				}
				log.Printf("New connection: FD %d", nfd)

				// set new client FD to non-blocking and add to epoll
				syscall.SetNonblock(nfd, true)
				ev := syscall.EpollEvent{
					Events: uint32(syscall.EPOLLIN) | 0x80000000, // EPOLLET: edge-triggered
					Fd:     int32(nfd),
				}
				syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, nfd, &ev)

			} else {
				// --- Existing client sent data ---
				if !handleEcho(fd) {
					// On error or disconnect, remove from epoll and close
					syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, fd, nil)
					syscall.Close(fd)
					log.Printf("Closed connection: FD %d", fd)
				}
			}
		}
	}
}

func handleEcho(fd int) bool {
	buf := make([]byte, 1024)
	// Non-blocking Read
	n, err := syscall.Read(fd, buf)
	if n <= 0 {
		return false // Disconnected
	}

	fmt.Printf("[FD %d] Received: %s", fd, string(buf[:n]))

	// Non-blocking Write (simplified, partial write handling omitted)
	_, err = syscall.Write(fd, buf[:n])
	return err == nil
}
