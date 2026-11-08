package main

import (
	"context"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

// discoverLocalPorts only probes 127.0.0.1. This discovers an existing
// TCP listener; it cannot start the Workbench scripting service if stopped.
// A TCP listener is not proof of a Workbench server: wbRun("list") verifies it.
func discoverLocalPorts(preferred int, low, high int) []int {
	if low < 1024 {
		low = 1024
	}
	if high > 65535 {
		high = 65535
	}
	if high < low {
		return nil
	}
	workers := 64
	if high-low+1 < workers {
		workers = high - low + 1
	}
	jobs := make(chan int)
	results := make(chan int, high-low+1)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), 110*time.Millisecond)
				conn, e := (&net.Dialer{}).DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
				cancel()
				if e == nil {
					conn.Close()
					results <- p
				}
			}
		}()
	}
	go func() {
		for p := low; p <= high; p++ {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	ports := make([]int, 0, 16)
	for p := range results {
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool {
		x, y := absPort(ports[i]-preferred), absPort(ports[j]-preferred)
		if x != y {
			return x < y
		}
		return ports[i] < ports[j]
	})
	return ports
}
func absPort(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
