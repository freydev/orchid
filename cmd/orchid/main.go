package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	handler, err := route(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		usage()
		return 2
	}
	return handler()
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: orchid <command> [args]

commands:
  serve [-host H -port P]   run the server in the foreground
  start [-host H -port P]   start the server as a background daemon
  stop                      stop the daemon
  status                    show daemon status and session count
  open                      open the dashboard in a browser
  ls                        list sessions
  create <agent> [cwd]      create a session
  rm <id>                   delete a session
  restart <id>              restart a stopped session
  install-service           run the daemon at login (launchd / systemd user)
  uninstall-service         remove the login service
`)
}
