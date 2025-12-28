package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/treefix50/primetime/internal/ffmpeg"
	"github.com/treefix50/primetime/internal/server"
	"github.com/treefix50/primetime/internal/storage"
)

func main() {
	var (
		root = flag.String("root", "./media", "media root directory")
		addr = flag.String("addr", ":8080", "listen address")
		db   = flag.String("db", defaultDBPath(), "sqlite database path")
		scan = flag.String("scan-interval", "10m", "media scan interval (e.g. 10m, 0 to disable)")
		cors = flag.Bool("cors", false, "enable CORS headers for API responses")
	)
	flag.Parse()

	scanInterval, err := time.ParseDuration(*scan)
	if err != nil {
		log.Fatalf("level=error msg=\"invalid scan interval\" scan=%q err=%v", *scan, err)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("level=error msg=\"failed to get working directory\" err=%v", err)
	}

	ff, err := ffmpeg.Ensure(context.Background(), wd)
	if err != nil {
		log.Fatalf("level=error msg=\"failed to ensure ffmpeg\" err=%v", err)
	}
	log.Printf("level=info msg=\"ffmpeg ready\" path=%s", ff)

	if err := ensureDBDir(*db); err != nil {
		log.Fatalf("level=error msg=\"failed to ensure db path\" path=%s err=%v", *db, err)
	}

	store, err := storage.Open(*db)
	if err != nil {
		log.Fatalf("level=error msg=\"failed to open storage\" path=%s err=%v", *db, err)
	}

	s, err := server.New(*root, *addr, store, scanInterval, *cors)
	if err != nil {
		log.Fatalf("level=error msg=\"failed to initialize server\" addr=%s root=%s err=%v", *addr, *root, err)
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		log.Println("level=info msg=\"shutting down\"")
		_ = store.Close()
		_ = s.Close()
	}()

	log.Printf("level=info msg=\"server listening\" addr=%s root=%s url=http://localhost%s", *addr, *root, *addr)
	log.Fatalf("level=error msg=\"server stopped\" err=%v", s.Start())
}

func defaultDBPath() string {
	return "./data/primetime.db"
}

func ensureDBDir(path string) error {
	if path == ":memory:" {
		return nil
	}
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return fmt.Errorf("db path points to a directory")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return file.Close()
}
