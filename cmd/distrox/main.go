package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ziyasal/distroxy/internal/pkg/app"

	"github.com/ziyasal/distroxy/internal/pkg/common"

	"github.com/ziyasal/distroxy/pkg/distrox"
)

const (
	defaultGracefulShutdownTimeout = 15 * time.Second
	// server version
	version = "1.0.0"

	exitWithErr = 1
)

func main() {
	exit, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit = exitWithErr
	}

	// Exit with success status.
	os.Exit(exit)
}

func run() (int, error) {
	config, err := loadConfig()

	if err != nil {
		return exitWithErr, fmt.Errorf("couldn't load config: %s", err)
	}

	logger := common.NewZeroLogger(config.app.mode)
	cache, err := distrox.NewCache(
		distrox.WithMaxBytes(config.cache.maxBytes),
		distrox.WithShards(config.cache.shards),
		distrox.WithMaxKeySize(config.cache.maxKeySizeInBytes),
		distrox.WithMaxValueSize(config.cache.maxValueSizeInBytes),
		distrox.WithTTL(config.cache.ttlInSeconds),
		distrox.WithLogger(logger),
		distrox.WithStatsEnabled(),
	)
	if err != nil {
		return exitWithErr, err
	}

	logger.Info(fmt.Sprintf("Starting Distrox server(v%s) ...", version))

	srv := app.NewServer(fmt.Sprintf("%s:%d", config.app.host, config.app.port),
		cache,
		app.WithLogger(logger),
		app.WithPprof(config.app.pprofEnabled),
		app.WithMode(config.app.mode),
	)

	go func() {
		err := srv.Run()
		if err != nil {
			logger.Fatal("Startup failed", err)
		}
	}()

	exitCode := gracefullyShutdown(srv, logger)

	return exitCode, nil
}

func gracefullyShutdown(srv *app.Server, logger common.Logger) int {
	sig := make(chan os.Signal, 1)
	exit := 0
	defer close(sig)

	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Block and receive signal
	in := <-sig

	errCh := make(chan error)
	defer close(errCh)

	logger.Info(fmt.Sprintf("Received signal: %s, shutting down", in.String()))

	go func(s *app.Server, errCh chan error) {
		timeout, cancel := context.WithTimeout(context.Background(), defaultGracefulShutdownTimeout)
		defer cancel()

		if err := s.Shutdown(timeout); err != nil {
			errCh <- err
		}

		errCh <- nil
	}(srv, errCh)

	// Receive message from shutdown
	e := <-errCh

	if e != nil {
		// EX_SOFTWARE. See "sysexits.h"
		exit = 70

		logger.Err("Failed to shutdown server gracefully", e)
	} else {
		logger.Info("Successfully closed server")
	}

	// Terminate server
	logger.Info("Shutdown complete")

	return exit
}
