/*
   main.go

   Client for the final server.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

import (
	"fmt"
	"log"
	"net/rpc"
	"sort"
)

type RequestStats struct {
	TotalBytes              uint64
	RequestCount            uint32
	ResponseTotalMillis     uint64
	ResponseFirstByteMillis uint64
}

type Empty struct{}
type RequestStatsResponse struct {
	Requests map[string]RequestStats
}

func main() {
	client, err := rpc.DialHTTP("tcp", "127.0.0.1:8079")
	if err != nil {
		log.Fatalf("Failed to dial: %s", err)
	}

	var reply RequestStatsResponse
	err = client.Call("LcaProxy.GetRequestStats", &Empty{}, &reply)
	if err != nil {
		log.Fatalf("Failed to get stats: %s", err)
	}

	fmt.Printf("request statistics\n\n")
	fmt.Printf("count | total bytes | avg bytes | avg millis | FB millis | URL\n")
	fmt.Printf("-----------------------------------------------------------------------\n")

	paths := make(sort.StringSlice, 0)
	for path, _ := range reply.Requests {
		paths = append(paths, path)
	}
	sort.Sort(paths)

	for _, path := range paths {
		stats := reply.Requests[path]
		fmt.Printf("%5d | %11d | %9d | %10.2f | %9.2f | %s\n",
			stats.RequestCount, stats.TotalBytes,
			uint64(float64(stats.TotalBytes)/float64(stats.RequestCount)),
			float64(stats.ResponseTotalMillis)/float64(stats.RequestCount),
			float64(stats.ResponseFirstByteMillis)/float64(stats.RequestCount),
			path)
	}
}
