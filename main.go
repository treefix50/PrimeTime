package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/treefix50/primetime/internal/ffmpeg"
	"github.com/treefix50/primetime/internal/server"
)

func main() {
	var (
		root = flag.String("root", "./media", "media root directory")
		addr = flag.String("addr", ":8080", "listen address")
	)
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	ff, err := ffmpeg.Ensure(context.Background(), wd)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("ffmpeg: %s\n", ff)

	s, err := server.New(*root, *addr)
	if err != nil {
		log.Fatal(err)
	}

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
