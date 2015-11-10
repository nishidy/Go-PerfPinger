package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func main() {

	hostname := os.Args[1]
	ra, err := net.ResolveIPAddr("ip4", hostname)
	if err != nil {
		fmt.Println(os.Stderr, "Failed to resolve with ", hostname, err)
		os.Exit(1)
	}
	fmt.Println(ra)

	datalen, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println(os.Stderr, "Failed to convert with ", os.Args[2])
		os.Exit(2)
	}

	conn, err := icmp.ListenPacket("udp4", "")
	if err != nil {
		fmt.Println(os.Stderr, "Failed to listen upd4")
		os.Exit(3)
	}
	defer conn.Close()

	data := make([]byte, datalen)
	for i := 0; i < datalen; i++ {
		data[i] = 1
	}

	bytes, err := (&icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID: 0, Seq: 0,
			Data: data,
		},
	}).Marshal(nil)
	if err != nil {
		fmt.Println(os.Stderr, "Failed to create ICMP message")
		os.Exit(4)
	}

	size, err := conn.WriteTo(bytes, &net.UDPAddr{IP: ra.IP, Zone: ra.Zone})
	if err != nil {
		fmt.Println(os.Stderr, "Failed to write ICMP message", err)
		os.Exit(5)
	}

	conn.SetReadDeadline(time.Now().Add(time.Millisecond * 500))
	size, addr, err := conn.ReadFrom(bytes)
	if err != nil {
		fmt.Println(os.Stderr, "Failed to read ICMP message", err)
		os.Exit(6)
	}

	fmt.Println(size, addr, bytes)

}
