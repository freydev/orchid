package main

import "testing"

func TestRouteKnownCommands(t *testing.T) {
	for _, name := range []string{"serve", "start", "stop", "status", "open", "ls", "create", "rm", "restart"} {
		h, err := route([]string{name})
		if err != nil || h == nil {
			t.Fatalf("route(%q): got nil=%v err=%v; want handler", name, h == nil, err)
		}
	}
}

func TestRouteNoArgsIsServe(t *testing.T) {
	h, err := route(nil)
	if err != nil || h == nil {
		t.Fatalf("route(nil): got nil=%v err=%v; want serve handler", h == nil, err)
	}
}

func TestRouteFlagsAreServe(t *testing.T) {
	h, err := route([]string{"-port", "9000"})
	if err != nil || h == nil {
		t.Fatalf("route(-port): got nil=%v err=%v; want serve handler", h == nil, err)
	}
}

func TestRouteUnknown(t *testing.T) {
	if _, err := route([]string{"bogus"}); err == nil {
		t.Fatal("unknown command should error")
	}
}
