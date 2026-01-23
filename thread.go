package main

/*
#include <pthread.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>

// a structure to hold arguments for the thread
typedef struct {
    int fd;
} thread_arg_t;

// connection handler function for each thread, in C
void* echo_handler(void* arg) {
    thread_arg_t* t_arg = (thread_arg_t*)arg;
    int fd = t_arg->fd;
    free(t_arg);

    char buf[1024];
    ssize_t n;

    // read data from the connection
    while ((n = read(fd, buf, sizeof(buf))) > 0) {
        printf("[C-Thread %p] Received %zd bytes: %.*s\n", (void*)pthread_self(), n, (int)n, buf);

        // echo back the received data
        write(fd, buf, n);
    }

    if (n == 0) {
        printf("[C-Thread %p] Client closed connection (FD: %d)\n", (void*)pthread_self(), fd);
    } else if (n < 0) {
        perror("read error");
    }

    close(fd);
    return NULL;
}

// create a new pthread to handle the connection
void create_echo_thread(int fd) {
    pthread_t thread;
    thread_arg_t* arg = malloc(sizeof(thread_arg_t));
    arg->fd = fd;

	// create the thread
    if (pthread_create(&thread, NULL, echo_handler, arg) != 0) {
        perror("pthread_create failed");
        close(fd);
        free(arg);
        return;
    }

    // automatically reclaim resources when the thread exits
    pthread_detach(thread);
}
*/
import "C"
import (
	"log"
	"net"
	"os"
	"syscall"
)

func threadServer(addr string) {
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

	log.Printf("Pthread Server (PID: %d) started. Log will show in this terminal.\n", os.Getpid())

	for {
		nfd, _, err := syscall.Accept(listenFd)
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		// create a new pthread to handle the connection
		C.create_echo_thread(C.int(nfd))
	}

}
