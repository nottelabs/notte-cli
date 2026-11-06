// ui-preview runs the CLI output playground without contacting a live API.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nottelabs/notte-cli/internal/playground"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:4173", "Loopback address for the local playground")
	export := flag.String("export", "", "Export a static HTML site to this directory and exit")
	flag.Parse()
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "cmd", "notte", "main.go")); err != nil {
		return fmt.Errorf("run ui-preview from the notte-cli repository root: %w", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	server, err := playground.NewServer(root)
	if err != nil {
		return err
	}
	defer func() { _ = server.Close() }()
	if *export != "" {
		revision, err := playground.SourceRevision(root)
		if err != nil {
			return err
		}
		if err := server.Rebuild(ctx, revision); err != nil {
			return err
		}
		if err := playground.Export(*export, server.Snapshot(), playground.Page()); err != nil {
			return err
		}
		fmt.Printf("Exported playground to %s\n", *export)
		return nil
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("the development server must bind to a loopback address; use -export for a public static site")
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	httpServer := &http.Server{
		Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		server.Watch(ctx)
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	fmt.Printf("Notte CLI playground: http://%s\n", listener.Addr())
	fmt.Println("Synthetic fixtures only. Watching Go source, fixtures, and HTML for changes.")
	err = httpServer.Serve(listener)
	cancel()
	<-watchDone
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
