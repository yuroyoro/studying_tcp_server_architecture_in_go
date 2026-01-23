package main

import (
	"log"
	"os"
	"strconv"
)

// sample TCP echo server implementation
func main() {

	var addr = "0.0.0.0:" // wildcard network address and use ephemeral port

	mode := os.Args[1]

	if len(os.Args) > 2 && os.Args[2] != "" {
		port := os.Args[2]
		if _, err := strconv.Atoi(port); err == nil {
			addr = "0.0.0.0:" + port
		}
	}

	pid := os.Getpid()
	log.Printf("Launch Server : pid = %d, mode = %s, addr = %s", pid, mode, addr)

	switch mode {
	case "simple":
		simpleServer(addr)
	case "fork":
		forkServer(addr)
	case "prefork":
		preforkServer(addr)
	case "thread":
		threadServer(addr)
	case "asyncio":
		asyncioServer(addr)
	case "hybrid":
		hybridServer(addr)
	case "microthread":
		microthreadServer(addr)
	default:
		log.Panic("invalid mode")
	}
}
