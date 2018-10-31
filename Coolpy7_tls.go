package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
)

func main() {
	var (
		addr    = flag.String("listen", ":8883", "port to listen")
		tcpAddr = flag.String("tcp_addr", "127.0.0.1:1883", "ws tcp addr to proxy pass")
	)
	flag.Parse()

	if conn, err := net.Dial("tcp", *tcpAddr); err != nil {
		log.Fatalf("warning: test upstream error: %v", err)
	} else {
		log.Printf("upstream %s ok", *tcpAddr)
		conn.Close()
	}

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(err)
	}
	dir = dir + "/data"
	cert, err := tls.LoadX509KeyPair(dir+"/server.pem", dir+"/server.key")
	if err != nil {
		log.Fatal(err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	//Create incoming connections listener.
	ln, err := tls.Listen("tcp", *addr, config)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Println(err)
				continue
			}
			go upStreamTcpTls("tcp", *tcpAddr, conn)
		}
	}()

	log.Printf("proxy is listening on %q", *addr)
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			ln.Close()
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}

func upStreamTcpTls(network, addr string, conn net.Conn) {
	peer, err := net.Dial(network, addr)
	if err != nil {
		log.Printf("dial upstream error: %v", err)
		return
	}

	log.Printf("serving %s < %s <~> %s > %s", peer.RemoteAddr(), peer.LocalAddr(), conn.RemoteAddr(), conn.LocalAddr())

	go func() {
		if _, err := io.Copy(peer, conn); err != nil {
			peer.Close()
			conn.Close()
			return
		}
	}()
	go func() {
		if _, err := io.Copy(conn, peer); err != nil {
			peer.Close()
			conn.Close()
			return
		}
	}()
}