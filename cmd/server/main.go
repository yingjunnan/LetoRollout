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

	"letorollout/internal/auth"
	"letorollout/internal/config"
	"letorollout/internal/httpapi"
	"letorollout/internal/kube"
	"letorollout/internal/rollout"
)

func main() {
	cfg := config.Load()

	store, err := auth.LoadStore(cfg.TokensPath)
	if err != nil {
		log.Fatalf("load token store: %v", err)
	}

	var service httpapi.Service
	if cfg.LocalPreview {
		preview := httpapi.NewPreviewService()
		// seed a couple of fake deployments so the console has something to show
		preview.SeedDeployment(rollout.DeploymentSummary{
			Name: "api", Namespace: "default",
			Replicas: 2, ReadyReplicas: 2,
			Containers: []rollout.ContainerInfo{{Name: "app", Image: "nginx:1.27.0"}},
		})
		preview.SeedDeployment(rollout.DeploymentSummary{
			Name: "web", Namespace: "default",
			Replicas: 3, ReadyReplicas: 1,
			Containers: []rollout.ContainerInfo{
				{Name: "web", Image: "redis:7.2"},
				{Name: "sidecar", Image: "busybox:1.36"},
			},
		})
		service = preview
		log.Printf("letorollout console preview enabled on %s", cfg.Addr)
	} else {
		updater, err := kube.NewInClusterDeploymentImageUpdater(kube.UpdaterOptions{
			AllowedNamespaces:       cfg.AllowedNamespaces,
			RequiredDeploymentLabel: cfg.RequiredDeploymentLabel,
		})
		if err != nil {
			log.Fatalf("create deployment image updater: %v", err)
		}
		service = updater
	}

	handler := httpapi.NewHandler(httpapi.Config{
		AdminToken:   cfg.AdminToken,
		LogTailLines: cfg.LogTailLines,
	}, service, store)

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
