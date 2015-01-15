/*
   server.go

   Data structures and methods for the RPC server component of the project.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

type Empty struct{}
type RequestStatsResponse struct {
	Requests map[string]RequestStats
}

func (p *LcaProxy) GetRequestStats(args *Empty, reply *RequestStatsResponse) error {
	p.RequestLock.Lock()
	defer p.RequestLock.Unlock()

	reply.Requests = make(map[string]RequestStats)
	for k, v := range p.Requests {
		reply.Requests[k] = *v
	}
	return nil
}
