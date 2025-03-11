package filemanager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// FileManager handles operations on config files
type FileManager struct {
	configFile     string
	backupDir      string
	logger         *zap.Logger
	lastContentHash string
}

// NewFileManager creates a new FileManager
func NewFileManager(configFile, backupDir string, logger *zap.Logger) *FileManager {
	return &FileManager{
		configFile: configFile,
		backupDir:  backupDir,
		logger:     logger,
	}
}

// HasConfigChanged checks if the provided content is different from the current config file
func (fm *FileManager) HasConfigChanged(content string) (bool, error) {
	// Calculate hash of the new content
	contentHash := calculateHash(content)
	
	// If we already checked this content and it's unchanged, skip
	if contentHash == fm.lastContentHash {
		fm.logger.Debug("Config content hash matches last hash, no change detected")
		return false, nil
	}
	
	// If the file doesn't exist, it has changed
	fileInfo, err := os.Stat(fm.configFile)
	if os.IsNotExist(err) {
		fm.logger.Debug("Config file doesn't exist, treating as changed")
		fm.lastContentHash = contentHash
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat config file: %w", err)
	}
	
	// If the file is empty, it has changed
	if fileInfo.Size() == 0 {
		fm.logger.Debug("Config file is empty, treating as changed")
		fm.lastContentHash = contentHash
		return true, nil
	}
	
	// Read the current file content
	currentContent, err := os.ReadFile(fm.configFile)
	if err != nil {
		return false, fmt.Errorf("failed to read current config file: %w", err)
	}
	
	// Calculate hash of the current content
	currentHash := calculateHash(string(currentContent))
	
	// Check if the content has changed
	hasChanged := currentHash != contentHash
	
	// Update last content hash
	fm.lastContentHash = contentHash
	
	if hasChanged {
		fm.logger.Debug("Config content has changed", 
			zap.String("new_hash", contentHash[:8]),
			zap.String("old_hash", currentHash[:8]))
	} else {
		fm.logger.Debug("Config content unchanged")
	}
	
	return hasChanged, nil
}

// WriteConfigFile writes the content to the config file atomically
func (fm *FileManager) WriteConfigFile(content string) error {
	// Create temporary file in the same directory as the target file
	dir := filepath.Dir(fm.configFile)
	tempFile, err := os.CreateTemp(dir, "nats-config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFilePath := tempFile.Name()

	// Clean up the temporary file if something goes wrong
	defer func() {
		tempFile.Close()
		if _, err := os.Stat(tempFilePath); err == nil {
			os.Remove(tempFilePath)
		}
	}()

	// Write the content to the temporary file
	if _, err := tempFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Close the file to ensure all data is flushed to disk
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Create a backup of the current config file if it exists
	if err := fm.backupCurrentConfig(); err != nil {
		fm.logger.Warn("Failed to create backup", zap.Error(err))
		// Continue even if backup fails
	}

	// Atomically rename the temporary file to the target file
	if err := os.Rename(tempFilePath, fm.configFile); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	// Ensure proper file permissions
	if err := os.Chmod(fm.configFile, 0644); err != nil {
		fm.logger.Warn("Failed to set config file permissions", zap.Error(err))
		// Continue even if permission setting fails
	}

	fm.logger.Info("Successfully wrote config file", zap.String("path", fm.configFile))
	return nil
}

// backupCurrentConfig creates a backup of the current config file
func (fm *FileManager) backupCurrentConfig() error {
	// Check if the config file exists
	if _, err := os.Stat(fm.configFile); os.IsNotExist(err) {
		// No file to backup
		return nil
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(fm.backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupFilename := filepath.Join(fm.backupDir, fmt.Sprintf("nats-config-%s.conf", timestamp))

	// Open source file
	source, err := os.Open(fm.configFile)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	// Create destination file
	destination, err := os.Create(backupFilename)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer destination.Close()

	// Copy file contents
	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	fm.logger.Info("Created config backup", zap.String("backup", backupFilename))
	return nil
}

// ReadConfigFile reads the current config file content
func (fm *FileManager) ReadConfigFile() (string, error) {
	// Check if the file exists
	if _, err := os.Stat(fm.configFile); os.IsNotExist(err) {
		return "", nil // Return empty string if file doesn't exist
	}
	
	// Read the file
	content, err := os.ReadFile(fm.configFile)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}
	
	return string(content), nil
}

// CleanupOldBackups removes backups older than a certain age
func (fm *FileManager) CleanupOldBackups(maxAge time.Duration) error {
	// Get all files in the backup directory
	files, err := os.ReadDir(fm.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Backup directory doesn't exist, nothing to clean up
		}
		return fmt.Errorf("failed to read backup directory: %w", err)
	}
	
	// Get current time
	now := time.Now()
	
	// Check each file
	for _, file := range files {
		// Skip directories
		if file.IsDir() {
			continue
		}
		
		// Get file info
		fileInfo, err := file.Info()
		if err != nil {
			fm.logger.Warn("Failed to get file info", zap.String("file", file.Name()), zap.Error(err))
			continue
		}
		
		// Check if file is older than maxAge
		if now.Sub(fileInfo.ModTime()) > maxAge {
			// Remove the file
			filePath := filepath.Join(fm.backupDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				fm.logger.Warn("Failed to remove old backup", zap.String("file", filePath), zap.Error(err))
			} else {
				fm.logger.Debug("Removed old backup", zap.String("file", filePath))
			}
		}
	}
	
	return nil
}

// calculateHash calculates the SHA-256 hash of a string
func calculateHash(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return hex.EncodeToString(hasher.Sum(nil))
}
