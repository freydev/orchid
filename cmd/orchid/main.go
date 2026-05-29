package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"strconv"

	"orchid/internal/server"
	"orchid/internal/session"
)

func main() {
	host := flag.String("host", "127.0.0.1", "listen host (localhost only by default)")
	port := flag.Int("port", 8081, "listen port")
	flag.Parse()

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))

	mgr := session.NewManager()
	defer mgr.KillAll()

	h := server.New(mgr)
	log.Printf("orchid listening on http://%s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
