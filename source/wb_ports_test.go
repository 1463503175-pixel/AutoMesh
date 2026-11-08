package main

import (
	"net"

	"testing"
)

func TestAutoDiscoveryOnlyFindsLocalActiveListeners(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skip("loopback listener unavailable: " + err.Error())
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	ports := discoverLocalPorts(port, port, port)
	if len(ports) != 1 || ports[0] != port {
		t.Fatalf("expected local active listener %d, got %v", port, ports)
	}
}
func TestAutoDiscoveryRejectsInvalidRanges(t *testing.T) {
	if ports := discoverLocalPorts(10605, 20000, 19999); len(ports) != 0 {
		t.Fatal(ports)
	}
}
