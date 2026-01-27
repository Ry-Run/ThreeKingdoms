package main

import net "ThreeKingdoms"

func main() {
	addr := "127.0.0.1:8080"
	server := net.New(addr)
	server.Run()
}
