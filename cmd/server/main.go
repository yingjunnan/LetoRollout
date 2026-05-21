package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"letorollout/internal/config"
	"letorollout/internal/httpapi"
	"letorollout/internal/kube"
)

func main() {
	cfg := config.Load()

	updater, err := kube.NewInClusterDeploymentImageUpdater()
	if err != nil {
		log.Fatalf("create deployment image updater: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewHandler(updater, cfg.AuthToken),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("letorollout listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown http server: %v", err)
	}
}
