package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/treefix50/primetime/internal/server"
)

func main() {
	var (
		root = flag.String("root", "./media", "media root directory")
		addr = flag.String("addr", ":8080", "listen address")
	)
	flag.Parse()

	s, err := server.New(*root, *addr)
	if err != nil {
		log.Fatal(err)
	}

	// graceful-ish stop
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		log.Println("shutting down...")
		_ = s.Close()
	}()

	log.Printf("PrimeTime listening on http://localhost%s (root=%s)\n", *addr, *root)
	log.Fatal(s.Start())
}
