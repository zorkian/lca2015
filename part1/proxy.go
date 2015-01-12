/*
   proxy.go

   The main request handler logic. I.e., this is the code that will get called with a request
   object and needs to do something with it. "Business logic" goes here.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type RequestStats struct {
	TotalBytes              uint64
	RequestCount            uint32
	ResponseTotalMillis     uint64
	ResponseFirstByteMillis uint64
}

type LcaProxy struct {
	Addr        string
	Requests    map[string]*RequestStats
	RequestLock sync.Mutex
	Balancer    *Balancer
}

// GoToWork sets up the listening socket and starts waiting for connections. Only returns when
// the program is done.
func (p *LcaProxy) GoToWork() {
	ln, err := net.Listen("tcp", p.Addr)
	if err != nil {
		log.Fatalf("Failed to listen: %s", err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Failed to accept: %s", err)
			continue
		}
		go p.handleConnection(conn)
	}
}

// handleConnection is spawned once per connection from a client, and exits when the client is
// done sending requests.
func (p *LcaProxy) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		log.Printf("reading again")
		req, err := http.ReadRequest(reader)
		log.Printf("readed")
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed to read request: %s", err)
			}
			log.Printf("return")
			return
		}

		// This can't be done in a goroutine, since we want to block until the response is
		// received. HTTP/1.1 pipelining is not supported. We also are not using a buffered
		// writer to reduce time-to-first-byte.
		log.Printf("request %s", req.URL.Path)
		p.handleRequest(writer, req)
		writer.Flush()
	}
}

// handleRequest performs a single request. This is assumed to be called in a given goroutine,
// but there might be other concurrent goroutines on this LcaProxy struct.
func (p *LcaProxy) handleRequest(w io.Writer, r *http.Request) {
	// Construct a response request and send it to the proxy input. Then we can read on the channel
	// which blocks until there is a response available.
	start := time.Now()
	respChan := make(chan *http.Response)
	p.Balancer.RequestQueue <- &BalanceRequest{
		Request:      r,
		ResponseChan: respChan,
	}

	// Read the response. When this returns we will have gotten first bytes (headers), but the body
	// might still be in transit.
	response := <-respChan
	firstByte := time.Now()

	// Now write the response to the writer.
	response.Write(w)
	finish := time.Now()

	// Now we can input our statistics since we know this request finished and we have the timing
	// data for it. Since we're using a shared structure, we need to lock. At this point though
	// the request has been sent to the user, so we don't impact the user experience time if we
	// end up contending for the lock.
	p.RequestLock.Lock()
	defer p.RequestLock.Unlock()

	stats, ok := p.Requests[r.URL.Path]
	if !ok {
		stats = &RequestStats{}
		p.Requests[r.URL.Path] = stats
	}
	stats.TotalBytes += uint64(response.ContentLength)
	stats.RequestCount++
	stats.ResponseFirstByteMillis += uint64(firstByte.Sub(start).Nanoseconds() / 1000000)
	stats.ResponseTotalMillis += uint64(finish.Sub(start).Nanoseconds() / 1000000)

	fmt.Printf("fbt time up to %0.5fms\n", float64(stats.ResponseFirstByteMillis)/float64(stats.RequestCount))
	fmt.Printf("avg time up to %0.5fms\n", float64(stats.ResponseTotalMillis)/float64(stats.RequestCount))
}
