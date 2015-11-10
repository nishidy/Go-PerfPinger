package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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

	datalen, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println(os.Stderr, "Failed to convert with ", os.Args[2])
		os.Exit(2)
	}

	interval, err := strconv.ParseInt(os.Args[3], 10, 64)
	if err != nil {
		fmt.Println(os.Stderr, "Failed to convert with ", os.Args[3])
		os.Exit(7)
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

	rand.Seed(time.Now().UnixNano())
	id := rand.Intn(65535)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT)

	sends := 0
	receives := 0

	exitchan := make(chan int)
	go func() {
		t := time.NewTicker(time.Duration(interval) * time.Millisecond)
		for {
			select {
			case <-t.C:
				bytes, err := (&icmp.Message{
					Type: ipv4.ICMPTypeEcho,
					Code: 0,
					Body: &icmp.Echo{
						ID: id, Seq: sends,
						Data: data,
					},
				}).Marshal(nil)
				if err != nil {
					fmt.Println(os.Stderr, "Failed to create ICMP message")
					os.Exit(4)
				}

				_, err = conn.WriteTo(bytes, &net.UDPAddr{IP: ra.IP, Zone: ra.Zone})
				if err != nil {
					fmt.Println(os.Stderr, "Failed to write ICMP message", err)
					os.Exit(5)
				}

				conn.SetReadDeadline(time.Now().Add(time.Duration(interval) * time.Millisecond))
				size, addr, err := conn.ReadFrom(bytes)
				if err != nil {
					fmt.Printf("Time out from %s: icmp_seq=%d\n", ra.IP, sends)
				} else {
					fmt.Printf("%d bytes from %s: icmp_seq=%d\n", size, addr, sends)
					receives += 1
				}
				sends += 1

			case <-sigchan:
				exitchan <- 1
				return

			}
		}
	}()

	start := time.Now()
	_ = <-exitchan
	finish := time.Now()

	dur := finish.Sub(start).Seconds() / time.Second.Seconds()
	thr := float64(receives) * (float64(datalen) * 8) / (dur * 1024 * 2)
	fmt.Printf("%d packets in %.2f sec : %.2f Kbps of both UL and DL.\n", receives, dur, thr)

}
