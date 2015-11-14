package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/internal/iana"
	"golang.org/x/net/ipv4"
)

type PingNet struct {
	addr *net.IPAddr
	conn *icmp.PacketConn
}

type PingData struct {
	id   int
	data []byte
	size int
	intv uint64
}

type PingNum struct {
	sends    int
	receives int
}

type PingChan struct {
	exitch chan int
	collch chan int
}

type Ping struct {
	PingNet
	PingData
	PingNum
	PingChan
}

type Reply struct {
	addr  net.Addr
	size  int
	bytes []byte
}

func (p *Ping) doPing() {

	fmt.Printf("PERFPINGER %s (ID:%d): %d data bytes\n", p.addr.IP, p.id, p.size)

	t := time.NewTicker(
		time.Duration(p.intv) * time.Millisecond,
	)

	wg := new(sync.WaitGroup)

	for {
		select {
		case <-t.C:

			p.sends += 1
			wbytes, err := (&icmp.Message{
				Type: ipv4.ICMPTypeEcho,
				Code: 0,
				Body: &icmp.Echo{
					ID: p.id, Seq: p.sends,
					Data: p.data,
				},
			}).Marshal(nil)
			if err != nil {
				fmt.Println(os.Stderr, "Failed to create ICMP message")
				os.Exit(4)
			}

			_, err = p.conn.WriteTo(wbytes, &net.IPAddr{IP: p.addr.IP})
			if err != nil {
				fmt.Println(os.Stderr, "Failed to write ICMP message", err)
				os.Exit(5)
			}

			start := time.Now().UnixNano()

			p.conn.SetReadDeadline(
				time.Now().Add(time.Duration(p.intv) * time.Millisecond * 2),
			)

			wg.Add(1)
			go func() {
				for {
					rbytes := make([]byte, p.size+8)
					size, addr, err := p.conn.ReadFrom(rbytes)
					stop := time.Now().UnixNano()

					if err != nil {
						fmt.Printf("* Request timeout from %s: icmp_seq=%d\n", p.addr.IP, p.sends)
						break
					} else {
						if p.addr.String() == addr.String() {
							if p.parseMessage(Reply{addr, size, rbytes}, stop-start) {
								p.receives += 1
							}
							break
						}
					}
				}
				wg.Done()
			}()
			wg.Wait()

		case <-p.exitch:
			p.collch <- p.receives
			return

		}
	}
}

func (p *Ping) parseMessage(r Reply, rtt int64) bool {
	rep, err := icmp.ParseMessage(iana.ProtocolICMP, r.bytes)
	if err != nil {
		fmt.Printf("* Reply error from %s: icmp_seq=%d -> %s\n", r.addr, p.sends, r.bytes)
		return false
	}

	switch pkt := rep.Body.(type) {
	case *icmp.Echo:
		if !(pkt.ID == p.id && pkt.Seq <= p.sends && bytes.Equal(pkt.Data, p.data)) {
			fmt.Printf("* Wrong data %d bytes from %s (ID=[%d,%d], Seq=[%d,%d]): icmp_seq=%d\n",
				r.size, r.addr, pkt.ID, p.id, pkt.Seq, p.sends, p.sends)
			return false
		}
	default:
		fmt.Printf("* Not echo received from %s: icmp_seq=%d\n", r.size, r.addr, p.sends)
		return false
	}

	fmt.Printf("%d bytes from %s: icmp_seq=%d time=%.2f ms\n", r.size, r.addr, p.sends, float64(rtt)/float64(time.Millisecond))
	return true
}

func main() {

	if len(os.Args) < 4 {
		os.Exit(10)
	}

	hostfile := os.Args[1]
	hdlr, err := os.Open(hostfile)
	if err != nil {
		fmt.Println(os.Stderr, "Failed to open with ", hostfile, err)
		os.Exit(11)
	}
	defer hdlr.Close()

	size, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println(os.Stderr, "Failed to convert with ", os.Args[2])
		os.Exit(2)
	}

	intv, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil {
		fmt.Println(os.Stderr, "Failed to convert with ", os.Args[3])
		os.Exit(7)
	}

	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = uint8(i)
	}

	rand.Seed(time.Now().UnixNano())

	exitch := make(chan int, 1)
	collch := make(chan int, 1)
	repch := make(chan int, 1)

	pingnum := 0
	scanner := bufio.NewScanner(hdlr)
	for scanner.Scan() {
		host := scanner.Text()

		addr, err := net.ResolveIPAddr("ip4", host)
		if err != nil {
			fmt.Println(os.Stderr, "Failed to resolve with ", host, err)
			os.Exit(1)
		}

		conn, err := icmp.ListenPacket("ip4:icmp", "")
		if err != nil {
			fmt.Println(os.Stderr, "Failed to listen ip4:icmp (%s)", err)
			os.Exit(3)
		}
		defer conn.Close()

		id := rand.Intn(65535)

		p := &Ping{
			PingNet{addr, conn},
			PingData{id, data, size, intv},
			PingNum{0, 0},
			PingChan{exitch, collch},
		}

		go p.doPing()

		pingnum += 1
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT)

	allrecs := 0
	go func() {
		_ = <-sigch
		close(exitch)
		for i := 0; i < pingnum; i++ {
			allrecs += <-collch
		}
		repch <- allrecs
	}()

	start := time.Now()
	_ = <-exitch
	finish := time.Now()
	rep := <-repch

	fmt.Println("")
	fmt.Println("--- perfpinger statistics ---")

	dur := finish.Sub(start).Seconds() / time.Second.Seconds()
	thr := float64(rep) * (float64(size) * float64(pingnum) * 8) / (dur * 1024 * 2)
	fmt.Printf("Total %d packets in %.2f sec : %.2f Kbps of both UL and DL.\n", rep, dur, thr)

}
