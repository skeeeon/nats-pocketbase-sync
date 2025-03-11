package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nats-pocketbase-sync/internal/config"
	"nats-pocketbase-sync/internal/filemanager"
	"nats-pocketbase-sync/internal/generator"
	"nats-pocketbase-sync/internal/nats"
	"nats-pocketbase-sync/internal/pocketbase"
	"nats-pocketbase-sync/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// Define command-line flags
	configPath := flag.String("config", "", "Path to the configuration file")
	flag.Parse()

	// Initialize the logger with console output only for now
	logger.Init(logger.LogConfig{
		Level:    "info",
		FilePath: "",
	})
	log := logger.GetLogger()
	defer logger.Sync()

	log.Info("Starting NATS PocketBase sync service")

	// Load configuration
	cfg, err := config.LoadConfig(*configPath, log)
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Re-initialize logger with configuration from config file
	logger.Init(logger.LogConfig{
		Level:    cfg.App.LogLevel,
		FilePath: cfg.App.LogFile,
	})
	log = logger.GetLogger()
	
	log.Info("Configuration loaded",
		zap.String("pb_url", cfg.PocketBase.URL),
		zap.String("nats_config", cfg.NATS.ConfigFile),
		zap.Int("sync_interval", cfg.App.SyncInterval))

	// Create PocketBase client
	pbClient := pocketbase.NewClient(
		cfg.PocketBase.URL,
		cfg.PocketBase.UserCollection,
		cfg.PocketBase.RoleCollection,
		log.With(zap.String("component", "pocketbase")),
	)

	// Set log level to debug temporarily for authentication troubleshooting
	log.With(zap.String("component", "pocketbase")).Debug(
		"Authenticating with PocketBase",
		zap.String("url", cfg.PocketBase.URL),
		zap.String("identity", cfg.PocketBase.AdminEmail),
	)

	// Authenticate with PocketBase
	if err := pbClient.Authenticate(cfg.PocketBase.AdminEmail, cfg.PocketBase.AdminPassword); err != nil {
		logger.Fatal("Failed to authenticate with PocketBase", zap.Error(err))
	}

	// Create file manager
	fileManager := filemanager.NewFileManager(
		cfg.NATS.ConfigFile,
		cfg.NATS.ConfigBackupDir,
		log.With(zap.String("component", "filemanager")),
	)

	// Create config generator
	generator := generator.NewGenerator(
		cfg.NATS.DefaultPermissions.Publish,
		cfg.NATS.DefaultPermissions.Subscribe,
		log.With(zap.String("component", "generator")),
	)

	// Create NATS reloader
	reloader := nats.NewReloader(
		cfg.NATS.ReloadCommand,
		log.With(zap.String("component", "reloader")),
	)

	// Set up signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Create a ticker for periodic syncing
	ticker := time.NewTicker(time.Duration(cfg.App.SyncInterval) * time.Second)
	defer ticker.Stop()

	// Run the initial sync
	if err := runSync(pbClient, generator, fileManager, reloader, log); err != nil {
		log.Error("Initial sync failed", zap.Error(err))
	}

	// Main loop
	log.Info("Entering main loop", zap.Int("sync_interval", cfg.App.SyncInterval))
	for {
		select {
		case <-ticker.C:
			// Run sync
			if err := runSync(pbClient, generator, fileManager, reloader, log); err != nil {
				log.Error("Sync failed", zap.Error(err))
			}

			// Cleanup old backups (keep backups for 30 days)
			if err := fileManager.CleanupOldBackups(30 * 24 * time.Hour); err != nil {
				log.Warn("Failed to clean up old backups", zap.Error(err))
			}

		case <-stop:
			log.Info("Shutting down gracefully")
			return
		}
	}
}

// runSync performs a single synchronization cycle
func runSync(
	pbClient *pocketbase.Client,
	generator *generator.Generator,
	fileManager *filemanager.FileManager,
	reloader *nats.Reloader,
	log *zap.Logger,
) error {
	log.Info("Starting sync cycle")

	// Get roles from PocketBase
	roles, err := pbClient.GetAllMqttRoles()
	if err != nil {
		return fmt.Errorf("failed to get roles: %w", err)
	}

	// Get users from PocketBase
	users, err := pbClient.GetAllMqttUsers()
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	// Generate NATS configuration
	config, err := generator.GenerateConfig(roles, users)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Check if config has changed
	changed, err := fileManager.HasConfigChanged(config)
	if err != nil {
		return fmt.Errorf("failed to check if config changed: %w", err)
	}

	// Only write and reload if the config has changed
	if changed {
		log.Debug("Configuration has changed, updating file and reloading NATS")
		
		// Write configuration file
		if err := fileManager.WriteConfigFile(config); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}

		// Reload NATS
		if err := reloader.ReloadConfig(); err != nil {
			return fmt.Errorf("failed to reload NATS: %w", err)
		}

		log.Info("Sync completed successfully with config changes")
	} else {
		log.Info("Sync completed, no config changes detected")
	}

	return nil
}
