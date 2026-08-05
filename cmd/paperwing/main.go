package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/paperwing/paperwing/internal/accounts"
	"github.com/paperwing/paperwing/internal/auth"
	"github.com/paperwing/paperwing/internal/config"
	"github.com/paperwing/paperwing/internal/httpapi"
	"github.com/paperwing/paperwing/internal/imapmon"
	"github.com/paperwing/paperwing/internal/secure"
	"github.com/paperwing/paperwing/internal/store"
	"github.com/paperwing/paperwing/internal/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			logger.Error("healthcheck failed", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := run(logger); err != nil {
		logger.Error("paperwing stopped", "error", err)
		os.Exit(1)
	}
}

func healthcheck() error {
	address := os.Getenv("PAPERWING_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse PAPERWING_ADDRESS: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", response.Status)
	}
	return nil
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	keyPath := filepath.Join(filepath.Dir(cfg.DatabasePath), "master.key")
	key, created, err := secure.LoadOrCreateKey(keyPath)
	if created {
		logger.Info("generated master key", "path", keyPath)
	}
	if err != nil {
		return err
	}
	cipher, err := secure.New(key)
	if err != nil {
		return err
	}
	repository, err := store.Open(cfg.DatabasePath, cipher)
	if err != nil {
		return err
	}
	defer repository.Close()

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	monitor := imapmon.NewManager(appCtx, repository, cfg.AttachmentPath, logger)
	defer monitor.Close()
	if err := monitor.Start(appCtx); err != nil {
		return err
	}
	accountService := accounts.New(repository, monitor, imapmon.TestConnection, cfg.AttachmentPath)
	authService := auth.New(repository)
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           web.New(httpapi.New(accountService, repository, authService, logger)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("paperwing listening", "address", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-appCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownWait)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
