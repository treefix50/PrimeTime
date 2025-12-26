package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/treefix50/primetime/internal/ffmpeg"
	"github.com/treefix50/primetime/internal/server"
	"github.com/treefix50/primetime/internal/storage"
)

func main() {
	var (
		root = flag.String("root", "./media", "media root directory")
		addr = flag.String("addr", ":8080", "listen address")
		db   = flag.String("db", defaultDBPath(), "sqlite database path")
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

	if err := ensureDBDir(*db); err != nil {
		log.Fatal(err)
	}

	store, err := storage.Open(*db)
	if err != nil {
		log.Fatal(err)
	}

	s, err := server.New(*root, *addr, store)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		log.Println("shutting down...")
		_ = store.Close()
		_ = s.Close()
	}()

	log.Printf("PrimeTime listening on http://localhost%s (root=%s)\n", *addr, *root)
	log.Fatal(s.Start())
}

func defaultDBPath() string {
	return "./data/primetime.db"
}

func ensureDBDir(path string) error {
	if path == ":memory:" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
