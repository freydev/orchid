package main

import (
	"flag"
	"log"
	"net/http"

	"orchid/internal/server"
	"orchid/internal/session"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (localhost only by default)")
	flag.Parse()

	mgr := session.NewManager()
	defer mgr.KillAll()

	h := server.New(mgr)
	log.Printf("orchid listening on http://%s", *addr)
	if err := http.ListenAndServe(*addr, h); err != nil {
		log.Fatal(err)
	}
}
