/*
   balancer.go

   Responsible for the logic of getting backends for our system to talk to. This is the main logic
   of the proxy. I.e., here is where you might implement your backend selection logic.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

import (
	"io/ioutil"
	"net/http"
	"time"
)

type BalanceRequest struct {
	Request      *http.Request
	ResponseChan chan *http.Response
}

type Balancer struct {
	Backends     []string
	RequestQueue chan *BalanceRequest
}

func MakeBalancer(backends []string) *Balancer {
	newBalancer := &Balancer{
		Backends:     backends,
		RequestQueue: make(chan *BalanceRequest),
	}

	go newBalancer.Balance()

	return newBalancer
}

// Balance is a permanent goroutine that reads requests and does something with them.
func (self *Balancer) Balance() {
	for {
		req := <-self.RequestQueue

		go func() {
			body := "this is a response"
			response := &http.Response{
				Status:        "200 OK",
				StatusCode:    200,
				Proto:         "HTTP/1.0",
				ProtoMajor:    1,
				ProtoMinor:    0,
				Body:          ioutil.NopCloser(MakeDelayedStartReader(0*time.Second, body)),
				ContentLength: int64(len(body)),
				Request:       req.Request,
			}

			//time.Sleep(1 * time.Second)

			req.ResponseChan <- response
		}()
	}
}
