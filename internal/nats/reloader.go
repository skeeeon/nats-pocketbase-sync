package nats

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Reloader handles reloading the NATS server configuration
type Reloader struct {
	reloadCommand string
	logger        *zap.Logger
	lastReload    time.Time
	mutex         sync.Mutex
	minInterval   time.Duration // Minimum time between reloads
}

// NewReloader creates a new NATS Reloader
func NewReloader(reloadCommand string, logger *zap.Logger) *Reloader {
	return &Reloader{
		reloadCommand: reloadCommand,
		logger:        logger,
		minInterval:   5 * time.Second, // Default minimum interval between reloads
	}
}

// ReloadConfig triggers a reload of the NATS server configuration
func (r *Reloader) ReloadConfig() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if we've reloaded recently
	if time.Since(r.lastReload) < r.minInterval {
		r.logger.Debug("Skipping reload, too soon since last reload")
		return nil
	}

	// Split command and arguments
	parts := strings.Fields(r.reloadCommand)
	if len(parts) == 0 {
		return fmt.Errorf("empty reload command")
	}

	// Extract command and arguments
	cmdName := parts[0]
	var cmdArgs []string
	if len(parts) > 1 {
		cmdArgs = parts[1:]
	}

	// Create command
	cmd := exec.Command(cmdName, cmdArgs...)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload command failed: %w, output: %s", err, string(output))
	}

	// Update last reload time
	r.lastReload = time.Now()

	r.logger.Info("Successfully reloaded NATS configuration", zap.String("output", string(output)))
	return nil
}

// SetMinimumInterval sets the minimum interval between reloads
func (r *Reloader) SetMinimumInterval(interval time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.minInterval = interval
}
