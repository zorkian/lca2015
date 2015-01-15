/*
   balancer.go

   Responsible for the logic of getting backends for our system to talk to. This is the main logic
   of the proxy. I.e., here is where you might implement your backend selection logic.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

type BackendConn struct {
	Conn   net.Conn
	Reader *bufio.Reader
}

type BalanceRequest struct {
	Request      *http.Request
	ResponseChan chan *BalanceResponse
}

type BalanceResponse struct {
	Response *http.Response
	Backend  *BackendConn
}

type Balancer struct {
	RequestQueue chan *BalanceRequest
	backends     []string
	backendQueue chan *BackendConn
}

func MakeBalancer(backends []string) *Balancer {
	newBalancer := &Balancer{
		// Size of this channel is how many pending requests can be in the queue. If this is hit,
		// we start throwing 503s to the user.
		RequestQueue: make(chan *BalanceRequest, 100),
		backends:     backends,
		// Size of this channel is how many spare backends we keep warm. If this queue is filled
		// then we start closing backend connections.
		backendQueue: make(chan *BackendConn, 300),
	}

	go newBalancer.BackendManager()
	go newBalancer.Balance()
	return newBalancer
}

// BackendManager creates backends for our proxy to use.
func (b *Balancer) BackendManager() {
	for {
		for _, addr := range b.backends {
			//log.Printf("Dialing %s...", addr)
			be, err := net.Dial("tcp", addr)
			if err != nil {
				log.Printf("Failed to dial %s: %s", addr, err)
				continue
			}
			b.backendQueue <- &BackendConn{Conn: be, Reader: bufio.NewReader(be)}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// BackendFinished is called by the proxy when it has finished using a backend. I.e., this is
// called after the response is read completely.
func (b *Balancer) BackendFinished(response *BalanceResponse) {
	// If the response indicated that things should be closed, then we know that we should
	// abandon this backend.
	if response.Response.Close {
		response.Backend.Conn.Close()
		return
	}

	// Safe to re-enqueue, probably. Try.
	select {
	case b.backendQueue <- response.Backend:
		// Do nothing, queued.
	case <-time.After(250 * time.Millisecond):
		response.Backend.Conn.Close()
	}
}

// Balance is a permanent goroutine that reads requests and does something with them.
func (b *Balancer) Balance() {
	for {
		req := <-b.RequestQueue

		// Get a backend. This blocks until one is available, but we send it off in a goroutine
		// so that we don't block the request pump.
		go func() {
			backend := <-b.backendQueue
			req.Request.Write(backend.Conn)
			resp, err := http.ReadResponse(backend.Reader, req.Request)
			if err != nil {
				req.ResponseChan <- &BalanceResponse{
					Response: MakeResponse(req.Request, 500, "500 Service Failure",
						fmt.Sprintf("%s", err)),
					Backend: backend,
				}
			}
			req.ResponseChan <- &BalanceResponse{Response: resp, Backend: backend}
		}()
	}
}
