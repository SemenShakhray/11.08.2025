package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"downloader/internal/config"
	"downloader/internal/handlers"
	"downloader/internal/logger"
	"downloader/internal/router"
	"downloader/internal/service"
)

func main() {
	cfg := config.MustLoad()

	log := logger.NewLogger()

	s := service.NewService(cfg, log)

	h := handlers.NewHandler(s, log)

	r := router.NewRouter(h)

	srv := &http.Server{
		Addr:         net.JoinHostPort(cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.Timeout,
		WriteTimeout: cfg.Server.Timeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	log.Info("start server", slog.String("host", cfg.Server.Host), slog.String("port", cfg.Server.Port))

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("failed to start server", slog.String("error", err.Error()))

			os.Exit(1)
		}
	}()

	sig := <-sigint
	log.Info("received signal", slog.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Info("failed to stop server", slog.String("error", err.Error()))
	}
}
