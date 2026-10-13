package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed web/* schema.sql
var bundled embed.FS

func main() {
	demo := flag.Bool("demo", false, "seed demonstration accounts and sample records when the database is new; development use only")
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (use localhost for classroom demo)")
	dbFile := flag.String("db", filepath.Join("data", "student_records.db"), "SQLite database file")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbFile), 0700); err != nil {
		log.Fatal(err)
	}
	store, err := openStore(*dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	initial, err := store.initialize(*demo)
	if err != nil {
		log.Fatal(err)
	}
	if initial != "" {
		fmt.Println("\nFIRST RUN ADMIN ACCOUNT")
		fmt.Println("Username: admin")
		fmt.Println("Password:", initial)
		fmt.Println("Save this password now; it will NOT be displayed again.")
	}
	if *demo {
		fmt.Println("Demo login (only seeded when database is new): admin / Admin@12345  |  s2026001 / Student@12345")
	}

	app := newApp(store)
	srv := &http.Server{Addr: *addr, Handler: app.routes(), ReadHeaderTimeout: 7 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	url := "http://" + strings.Replace(*addr, "0.0.0.0", "127.0.0.1", 1)
	log.Printf("Student Record System running: %s", url)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
