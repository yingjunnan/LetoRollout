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

	var handler http.Handler
	if cfg.LocalPreview {
		handler = httpapi.NewHandler(httpapi.NewPreviewService(), cfg.AdminToken)
		log.Printf("letorollout console preview enabled on %s", cfg.Addr)
	} else {
		updater, err := kube.NewInClusterDeploymentImageUpdater(kube.UpdaterOptions{
			AllowedNamespaces:       cfg.AllowedNamespaces,
			RequiredDeploymentLabel: cfg.RequiredDeploymentLabel,
		})
		if err != nil {
			log.Fatalf("create deployment image updater: %v", err)
		}
		handler = httpapi.NewHandler(updater, cfg.AdminToken)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
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
