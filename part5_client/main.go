/*
   main.go

   This is the kickoff/configuration source. In a real program you'd probably put flags here
   and maybe configuration file stuff.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

import (
	"log"
	"net/rpc"
	"sort"
)

type Empty struct{}
type Stats struct {
	RequestBytes map[string]int64
}
type RpcServer struct{}

type RequestStats struct {
	Path  string
	Bytes int64
}
type RequestStatsSlice []*RequestStats

func (r RequestStatsSlice) Less(i, j int) bool {
	return r[i].Bytes < r[j].Bytes
}

func (r RequestStatsSlice) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r RequestStatsSlice) Len() int {
	return len(r)
}

func main() {
	client, err := rpc.DialHTTP("tcp", "127.0.0.1:8079")
	if err != nil {
		log.Fatalf("Failed to dial: %s", err)
	}

	var reply Stats
	err = client.Call("RpcServer.GetStats", &Empty{}, &reply)
	if err != nil {
		log.Fatalf("Failed to GetStats: %s", err)
	}

	rss := make(RequestStatsSlice, 0)
	for k, v := range reply.RequestBytes {
		rss = append(rss, &RequestStats{Path: k, Bytes: v})
	}
	sort.Sort(rss)

	for i := len(rss) - 1; i > len(rss)-10; i-- {
		log.Printf("%10d %s", rss[i].Bytes, rss[i].Path)
	}
}
