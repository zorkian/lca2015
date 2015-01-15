/*
   main.go

   This is the kickoff/configuration source. In a real program you'd probably put flags here
   and maybe configuration file stuff.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"sync"
	"time"
)

type Backend struct {
	net.Conn
	Reader *bufio.Reader
	Writer *bufio.Writer
}

type Empty struct{}
type Stats struct {
	RequestBytes map[string]int64
}
type RpcServer struct{}

var backendQueue chan *Backend
var requestBytes map[string]int64
var requestLock sync.Mutex

func init() {
	requestBytes = make(map[string]int64)
	backendQueue = make(chan *Backend, 10)
}

func (r *RpcServer) GetStats(args *Empty, reply *Stats) error {
	requestLock.Lock()
	defer requestLock.Unlock()

	reply.RequestBytes = make(map[string]int64)
	for k, v := range requestBytes {
		reply.RequestBytes[k] = v
	}
	return nil
}

func main() {
	rpc.Register(&RpcServer{})
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", ":8079")
	if err != nil {
		log.Fatalf("Failed to listen: %s", err)
	}
	go http.Serve(l, nil)

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to listen: %s", err)
	}
	for {
		if conn, err := ln.Accept(); err == nil {
			go handleConnection(conn)
		}
	}
}

// handleConnection is spawned once per connection from a client, and exits when the client is
// done sending requests.
func handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed to read request: %s", err)
			}
			return
		}

		be, err := getBackend()
		if err != nil {
			return
		}

		if err := req.Write(be.Writer); err == nil {
			be.Writer.Flush()
			if resp, err := http.ReadResponse(be.Reader, req); err == nil {
				bytes := updateStats(req, resp)
				resp.Header.Set("X-Bytes", strconv.FormatInt(bytes, 10))

				FixHttp10Response(resp, req)
				if err := resp.Write(conn); err == nil {
					log.Printf("proxied %s: got %d", req.URL.Path, resp.StatusCode)
				}
				if resp.Close {
					return
				}
			}
		}

		go queueBackend(be)
	}
}

// updateStats takes a request and response and collects some statistics about them. This is
// very simple for now.
func updateStats(req *http.Request, resp *http.Response) int64 {
	requestLock.Lock()
	defer requestLock.Unlock()

	bytes := requestBytes[req.URL.Path] + resp.ContentLength
	requestBytes[req.URL.Path] = bytes
	return bytes
}

// getBackend either gets a backend from the queue or makes a new backend.
func getBackend() (*Backend, error) {
	select {
	case be := <-backendQueue:
		return be, nil
	case <-time.After(100 * time.Millisecond):
		be, err := net.Dial("tcp", "127.0.0.1:8081")
		if err != nil {
			return nil, err
		}

		return &Backend{
			Conn:   be,
			Reader: bufio.NewReader(be),
			Writer: bufio.NewWriter(be),
		}, nil
	}
}

// queueBackend takes a backend and reenqueues it.
func queueBackend(be *Backend) {
	select {
	case backendQueue <- be:
		// Backend re-enqueued safely, move on.
	case <-time.After(1 * time.Second):
		// If this fires, it means the queue of backends was full. We don't want to wait
		// forever, as this period of time blocks us handling the next request a user
		// might send us.
		be.Close()
	}
}
