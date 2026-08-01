package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"v2ray-server/internal/api"
	"v2ray-server/internal/config"
	"v2ray-server/internal/constants"
	"v2ray-server/internal/database"
	"v2ray-server/internal/repository"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func run() error {
	system := config.LoadSystemMeta()

	if err := utils.EnsureDirs(
		system.Home,
		filepath.Dir(system.Paths.Database),
		system.Paths.GeoDir,
		filepath.Dir(system.Paths.XrayConfigPath),
	); err != nil {
		return fmt.Errorf("init dirs: %w", err)
	}

	db, err := database.Init(system.Paths.Database)
	if err != nil {
		return fmt.Errorf("database init: %w", err)
	}

	settingRepo := repository.NewSettingRepository(db)
	cfg, err := config.Load(settingRepo)
	if err != nil {
		return fmt.Errorf("config init: %w", err)
	}

	services, err := service.Init(db, cfg)
	if err != nil {
		return fmt.Errorf("service init: %w", err)
	}

	services.EnsureGeoFiles()

	return startServer(cfg.SystemMeta(), db, services)
}

func startServer(system config.SystemMeta, db *gorm.DB, services *service.Container) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	api.SetupRoutes(r, services)
	registerStaticWebRoutes(r, system.Web.Root)

	addr := fmt.Sprintf("%s:%d", system.Server.Host, system.Server.Port)
	srv := &http.Server{Addr: addr, Handler: r}
	log.Printf("Server starting on %s", addr)

	serverErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErrCh:
		log.Printf("Server listen error: %v", err)
		cleanupRuntimeResources(db, services)
		return err
	case sig := <-quit:
		log.Printf("Shutting down server gracefully on %s...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), constants.ShutdownTimeout)
		defer cancel()

		shutdownErr := srv.Shutdown(ctx)
		if shutdownErr != nil {
			log.Printf("Server forced to shutdown: %v", shutdownErr)
		}

		cleanupRuntimeResources(db, services)
		if shutdownErr != nil {
			return shutdownErr
		}

		log.Println("Server exiting")
		return nil
	}
}

func cleanupRuntimeResources(db *gorm.DB, services *service.Container) {
	if err := services.Close(); err != nil {
		log.Printf("Service cleanup error: %v", err)
	}

	if err := database.Close(db); err != nil {
		log.Printf("Database cleanup error: %v", err)
	}
}

func registerStaticWebRoutes(r *gin.Engine, webRoot string) {
	if webRoot == "" {
		log.Println("Web root not configured; serving API only")
		return
	}

	indexPath := filepath.Join(webRoot, "index.html")

	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		log.Printf("Failed to load index.html from %s: %v", indexPath, err)
		return
	}

	log.Printf("Serving web assets from %s", webRoot)

	r.Static("/assets", filepath.Join(webRoot, "assets"))

	for _, file := range []string{
		"favicon.svg",
		"favicon.ico",
	} {
		path := filepath.Join(webRoot, file)
		if _, err := os.Stat(path); err == nil {
			r.StaticFile("/"+file, path)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "API route not found",
			})
			return
		}

		if c.Request.Method != http.MethodGet &&
			c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	})
}
