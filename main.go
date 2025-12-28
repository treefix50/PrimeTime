package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/treefix50/primetime/internal/ffmpeg"
	"github.com/treefix50/primetime/internal/server"
	"github.com/treefix50/primetime/internal/storage"
)

func main() {
	var (
		root           = flag.String("root", "./media", "media root directory")
		addr           = flag.String("addr", ":8080", "listen address")
		db             = flag.String("db", defaultDBPath(), "sqlite database path")
		dbReadOnly     = flag.Bool("db-read-only", false, "open sqlite database in read-only mode")
		dbBusyTimeout  = flag.Duration("db-busy-timeout", 5*time.Second, "sqlite busy timeout (e.g. 5s, 0 to disable)")
		dbSynchronous  = flag.String("db-synchronous", "NORMAL", "sqlite synchronous mode (OFF, NORMAL, FULL, EXTRA)")
		dbCacheSize    = flag.Int("db-cache-size", -65536, "sqlite cache size (negative values are KiB)")
		scan           = flag.String("scan-interval", "10m", "media scan interval (e.g. 10m, 0 to disable)")
		cors           = flag.Bool("cors", false, "enable CORS headers for API responses")
		integrityCheck = flag.Bool("sqlite-integrity-check", false, "run PRAGMA integrity_check and exit")
		vacuum         = flag.Bool("sqlite-vacuum", false, "run VACUUM and exit")
		vacuumInto     = flag.String("sqlite-vacuum-into", "", "run VACUUM INTO <path> and exit")
		analyze        = flag.Bool("sqlite-analyze", false, "run ANALYZE and exit")
	)
	flag.Parse()

	options := storage.Options{
		BusyTimeout: *dbBusyTimeout,
		Synchronous: *dbSynchronous,
		CacheSize:   *dbCacheSize,
		ReadOnly:    *dbReadOnly,
	}

	if err := runSQLiteMaintenance(*db, options, *integrityCheck, *vacuum, *vacuumInto, *analyze); err != nil {
		log.Fatalf("level=error msg=\"sqlite maintenance failed\" err=%v", err)
	}

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

	if *dbReadOnly {
		if err := ensureDBReadable(*db); err != nil {
			log.Fatalf("level=error msg=\"failed to verify db path\" path=%s err=%v", *db, err)
		}
	} else {
		if err := ensureDBDir(*db); err != nil {
			log.Fatalf("level=error msg=\"failed to ensure db path\" path=%s err=%v", *db, err)
		}
	}

	store, err := storage.Open(*db, options)
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

func ensureDBReadable(path string) error {
	if path == ":memory:" {
		return fmt.Errorf("read-only mode requires a file-backed database")
	}
	dbPath := dbFilePath(path)
	info, err := os.Stat(dbPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("db path points to a directory")
	}
	return nil
}

func dbFilePath(path string) string {
	if !strings.HasPrefix(path, "file:") {
		return path
	}
	parsed, err := url.Parse(path)
	if err != nil {
		return path
	}
	dbPath := parsed.Path
	if dbPath == "" {
		dbPath = parsed.Opaque
	}
	if dbPath == "" {
		return path
	}
	unescaped, err := url.PathUnescape(dbPath)
	if err != nil {
		return dbPath
	}
	return unescaped
}

func runSQLiteMaintenance(dbPath string, options storage.Options, integrityCheck, vacuum bool, vacuumInto string, analyze bool) error {
	if !integrityCheck && !vacuum && vacuumInto == "" && !analyze {
		return nil
	}
	if options.ReadOnly && (vacuum || vacuumInto != "" || analyze) {
		return fmt.Errorf("read-only mode does not allow sqlite vacuum/analyze")
	}
	if vacuum && vacuumInto != "" {
		return fmt.Errorf("choose either -sqlite-vacuum or -sqlite-vacuum-into, not both")
	}
	if options.ReadOnly {
		if err := ensureDBReadable(dbPath); err != nil {
			return err
		}
	} else {
		if err := ensureDBDir(dbPath); err != nil {
			return err
		}
	}
	if vacuumInto != "" {
		if err := ensureBackupTarget(vacuumInto); err != nil {
			return err
		}
	}

	store, err := storage.Open(dbPath, options)
	if err != nil {
		return err
	}
	defer func() {
		_ = store.Close()
	}()

	if integrityCheck {
		results, err := store.IntegrityCheck()
		if err != nil {
			return err
		}
		if len(results) == 0 {
			log.Printf("level=info msg=\"sqlite integrity check returned no rows\"")
		} else {
			for _, result := range results {
				log.Printf("level=info msg=\"sqlite integrity check\" result=%s", result)
			}
		}
	}
	if vacuum || vacuumInto != "" {
		target := vacuumInto
		if err := store.Vacuum(target); err != nil {
			return err
		}
		if target == "" {
			log.Printf("level=info msg=\"sqlite vacuum completed\"")
		} else {
			log.Printf("level=info msg=\"sqlite vacuum into completed\" path=%s", target)
		}
	}
	if analyze {
		if err := store.Analyze(); err != nil {
			return err
		}
		log.Printf("level=info msg=\"sqlite analyze completed\"")
	}
	os.Exit(0)
	return nil
}

func ensureBackupTarget(path string) error {
	if path == "" {
		return nil
	}
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return fmt.Errorf("backup path points to a directory")
		}
		return fmt.Errorf("backup file already exists")
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return nil
}
