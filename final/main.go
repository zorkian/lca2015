/*
   main.go

   This is the kickoff/configuration source. In a real program you'd probably put flags here
   and maybe configuration file stuff.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

func main() {
	proxy := &LcaProxy{
		Addr:     ":8080",
		RpcAddr:  ":8079",
		Requests: make(map[string]*RequestStats),
		Balancer: MakeBalancer([]string{"127.0.0.1:8081"}),
	}
	proxy.GoToWork()
}
