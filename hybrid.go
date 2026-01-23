package main

/*
#include <pthread.h>
#include <sys/epoll.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <fcntl.h>
#include <errno.h>

#define MAX_EVENTS 64

// Worker structure to hold epoll fd and thread id
typedef struct {
    int epfd;
    pthread_t thread_id;
} worker_t;

// Set FD to non-blocking mode
void set_nonblocking(int fd) {
    int flags = fcntl(fd, F_GETFL, 0);
    fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

// sub reactor loop running in each pthread
void* sub_reactor_loop(void* arg) {
    worker_t* worker = (worker_t*)arg;
    struct epoll_event events[MAX_EVENTS];
    char buf[1024];

    while (1) {
		// wait for events
        int n = epoll_wait(worker->epfd, events, MAX_EVENTS, -1);
        for (int i = 0; i < n; i++) {
            int fd = events[i].data.fd;

            // Edge-triggered (EPOLLET), so read until EAGAIN
            while (1) {
				// Non-blocking Read
                ssize_t count = read(fd, buf, sizeof(buf));
                if (count > 0) {
                    write(fd, buf, count); // Echo back
                    continue;
                }
                if (count == -1) {
                    if (errno == EAGAIN || errno == EWOULDBLOCK) break; // 読み切り
                    perror("read error");
                }

                // Disconnection or error
                epoll_ctl(worker->epfd, EPOLL_CTL_DEL, fd, NULL);
                close(fd);
                break;
            }
        }
    }
    return NULL;
}

// Initialize Worker and start pthread
worker_t* create_worker() {
    worker_t* w = malloc(sizeof(worker_t));
    w->epfd = epoll_create1(0);
    pthread_create(&w->thread_id, NULL, sub_reactor_loop, w);
    pthread_detach(w->thread_id);
    return w;
}

// Register FD to Worker from Main Reactor
void add_fd_to_worker(worker_t* w, int fd) {
    set_nonblocking(fd);
    struct epoll_event ev;
    ev.events = EPOLLIN | EPOLLET;
    ev.data.fd = fd;
    epoll_ctl(w->epfd, EPOLL_CTL_ADD, fd, &ev);
}
*/
import "C"

import (
	"log"
	"net"
	"runtime"
	"syscall"
)

func hybridServer(addr string) {
	// listen for incoming TCP connections
	// create tcp socket, bind to addr and start listening
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	// get raw FD from Listener.
	file, err := listener.(*net.TCPListener).File()
	if err != nil {
		log.Fatal(err)
	}
	listenFd := int(file.Fd())

	// 1. create cpu core number of C-pthread Workers
	numWorkers := runtime.NumCPU()
	workers := make([]*C.worker_t, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workers[i] = C.create_worker()
	}

	log.Printf("Multi-Reactor (C-pthread Workers) started on %s\n", addr)

	// 2. Main Reactor (Go): Accept connections and delegate to Workers
	counter := 0
	for {
		nfd, _, err := syscall.Accept(listenFd)
		if err != nil {
			continue
		}

		// Distribute FDs to Workers in round-robin fashion
		targetWorker := workers[counter%numWorkers]
		C.add_fd_to_worker(targetWorker, C.int(nfd))

		log.Printf("[Main] Accepted FD %d -> Assigned to Worker %d\n", nfd, counter%numWorkers)
		counter++
	}
}
