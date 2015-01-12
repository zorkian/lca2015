/*
   server.go

   Code related to the "server" side, i.e., the listener/request reception code goes here.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

import ()

func main() {
	proxy := &LcaProxy{
		Addr:     ":8080",
		Requests: make(map[string]*RequestStats),
		Balancer: MakeBalancer([]string{"127.0.0.1:8080"}),
	}
	proxy.GoToWork()
}
