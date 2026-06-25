// Command ccmanager is a localhost-only web tool for browsing, searching, and
// cleaning up the session transcripts and memory files Claude Code keeps under
// ~/.claude. See README.md for the full picture.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"ccmanager/internal/config"
	"ccmanager/internal/server"
	"ccmanager/internal/sessions"
	"ccmanager/internal/store"
	"ccmanager/internal/trash"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ccmanager:", err)
		os.Exit(1)
	}
}

func run() error {
	port := flag.Int("port", 8765, "port to listen on (bound to 127.0.0.1 only)")
	flag.Parse()

	cfg, err := config.Resolve(*port)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}
	if err := cfg.EnsureDataDirs(); err != nil {
		return err
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	tr := trash.New(st, cfg.TrashDir)
	if n, err := tr.PurgeExpired(time.Now()); err != nil {
		log.Printf("warning: purging expired trash: %v", err)
	} else if n > 0 {
		log.Printf("purged %d expired trash entries", n)
	}

	log.Printf("indexing transcripts under %s …", cfg.ProjectsDir)
	res, err := sessions.Index(st, cfg)
	if err != nil {
		return fmt.Errorf("initial index: %w", err)
	}
	log.Printf("indexed: %d scanned, %d unchanged, %d pruned, %d failed", res.Scanned, res.Skipped, res.Pruned, res.Failed)

	srv, err := server.New(cfg, st, tr)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", cfg.Port))
	log.Printf("ccmanager listening on http://%s", addr)
	httpSrv := &http.Server{Addr: addr, Handler: srv.Routes()}
	if err := httpSrv.ListenAndServe(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}
