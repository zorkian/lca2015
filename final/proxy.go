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
	"net/rpc"
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
	RpcAddr     string
	Requests    map[string]*RequestStats
	RequestLock sync.Mutex
	Balancer    *Balancer
}

// GoToWork sets up the listening socket and starts waiting for connections. Only returns when
// the program is done.
func (p *LcaProxy) GoToWork() {
	rpc.Register(p)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", p.RpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen: %s", err)
	}
	go http.Serve(l, nil)

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
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed to read request: %s", err)
			}
			return
		}

		// Intercept requests for /stats.
		if req.URL.Path == "/stats" {
			func() {
				p.RequestLock.Lock()
				defer p.RequestLock.Unlock()

				body := "request statistics\n\n"
				body += "count | total bytes | avg bytes | avg millis | FB millis | URL\n"
				body += "-----------------------------------------------------------------------\n"
				for path, stats := range p.Requests {
					body += fmt.Sprintf("%5d | %11d | %9d | %10.2f | %9.2f | %s\n",
						stats.RequestCount, stats.TotalBytes,
						uint64(float64(stats.TotalBytes)/float64(stats.RequestCount)),
						float64(stats.ResponseTotalMillis)/float64(stats.RequestCount),
						float64(stats.ResponseFirstByteMillis)/float64(stats.RequestCount),
						path)
				}

				resp := MakeResponse(req, 200, "200 OK", body)
				resp.Write(writer)
				writer.Flush()
			}()
			continue
		}

		// This can't be done in a goroutine, since we want to block until the response is
		// received. HTTP/1.1 pipelining is not supported. There is an optimization we would
		// normally make here in a real world HTTP proxy where we would not use a buffered
		// writer for the entire response, and would instead try to get the headers out ASAP so
		// the browser can start working.
		if !p.handleRequest(writer, req) {
			writer.Flush()
			return
		}
		writer.Flush()
	}
}

// handleRequest performs a single request. This is assumed to be called in a given goroutine,
// but there might be other concurrent goroutines on this LcaProxy struct. Returns whether or not
// we should continue accepting requests on this connection.
func (p *LcaProxy) handleRequest(w io.Writer, req *http.Request) bool {
	// Construct a response request and send it to the proxy input. Then we can read on the channel
	// which blocks until there is a response available.
	start := time.Now()
	respChan := make(chan *BalanceResponse)
	select {
	case p.Balancer.RequestQueue <- &BalanceRequest{
		Request:      req,
		ResponseChan: respChan,
	}:
		// Do nothing
	default:
		// Queue is full, throw an error. This is pretty janky.
		w.Write([]byte("HTTP/1.0 503 Service Unavailable\r\n\r\nQueue full."))
		return false
	}

	// Read the response. When this returns we will have gotten first bytes (headers), but the body
	// might still be in transit.
	response := <-respChan
	firstByte := time.Now()

	// We only speak 1.1 to our backends, so we have to downgrade their response to 1.0 if we got
	// a 1.0 request from the original user.
	if req.ProtoMajor == 1 && req.ProtoMinor == 0 {
		DowngradeResponse(response.Response, req)
	}

	// Now write the response to the writer.
	err := response.Response.Write(w)
	if err != nil {
		log.Printf("Failed to write response: %s", err)
	}
	finish := time.Now()

	// We are done with the backend now, release it to the balancer.
	p.Balancer.BackendFinished(response)

	// Now we can input our statistics since we know this request finished and we have the timing
	// data for it. Since we're using a shared structure, we need to lock. At this point though
	// the request has been sent to the user, so we don't impact the user experience time if we
	// end up contending for the lock.
	p.RequestLock.Lock()
	defer p.RequestLock.Unlock()

	stats, ok := p.Requests[req.URL.Path]
	if !ok {
		stats = &RequestStats{}
		p.Requests[req.URL.Path] = stats
	}
	stats.TotalBytes += uint64(response.Response.ContentLength)
	stats.RequestCount++
	stats.ResponseFirstByteMillis += uint64(firstByte.Sub(start).Nanoseconds() / 1000000)
	stats.ResponseTotalMillis += uint64(finish.Sub(start).Nanoseconds() / 1000000)

	//	fmt.Printf("fbt time up to %0.5fms\n", float64(stats.ResponseFirstByteMillis)/float64(stats.RequestCount))
	//	fmt.Printf("avg time up to %0.5fms\n", float64(stats.ResponseTotalMillis)/float64(stats.RequestCount))

	return !response.Response.Close
}
