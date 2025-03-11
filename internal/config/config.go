package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config represents the application configuration
type Config struct {
	App struct {
		SyncInterval int    `mapstructure:"sync_interval"`
		LogLevel     string `mapstructure:"log_level"`
		LogFile      string `mapstructure:"log_file"`
	} `mapstructure:"app"`

	PocketBase struct {
		URL            string `mapstructure:"url"`
		AdminEmail     string `mapstructure:"admin_email"`    // Username/email for the _superusers collection
		AdminPassword  string `mapstructure:"admin_password"` // Password for authentication
		UserCollection string `mapstructure:"user_collection"`
		RoleCollection string `mapstructure:"role_collection"`
	} `mapstructure:"pocketbase"`

	NATS struct {
		ConfigFile     string `mapstructure:"config_file"`
		ConfigBackupDir string `mapstructure:"config_backup_dir"`
		ReloadCommand  string `mapstructure:"reload_command"`
		DefaultPermissions struct {
			Publish   interface{} `mapstructure:"publish"`
			Subscribe interface{} `mapstructure:"subscribe"`
		} `mapstructure:"default_permissions"`
	} `mapstructure:"nats"`
}

// LoadConfig loads the configuration from config.yaml or environment variables
func LoadConfig(configPath string, logger *zap.Logger) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	
	// Set default config path if not provided
	if configPath == "" {
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
	} else {
		viper.AddConfigPath(configPath)
	}

	// Read environment variables with the prefix APP_
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	viper.SetDefault("app.sync_interval", 60)
	viper.SetDefault("app.log_level", "info")
	viper.SetDefault("app.log_file", "")
	viper.SetDefault("nats.config_backup_dir", "./backups")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("Config file not found, using defaults and environment variables")
		} else {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Ensure backup directory exists
	if _, err := os.Stat(cfg.NATS.ConfigBackupDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.NATS.ConfigBackupDir, 0755); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}
