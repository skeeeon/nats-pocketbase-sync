package generator

import (
	"fmt"
	"sort"
	"strings"

	"nats-pocketbase-sync/internal/models"
	"go.uber.org/zap"
)

// Generator handles the generation of NATS configuration from PocketBase data
type Generator struct {
	logger            *zap.Logger
	defaultPublish    interface{}
	defaultSubscribe  interface{}
}

// NewGenerator creates a new Generator
func NewGenerator(defaultPublish, defaultSubscribe interface{}, logger *zap.Logger) *Generator {
	return &Generator{
		logger:           logger,
		defaultPublish:   defaultPublish,
		defaultSubscribe: defaultSubscribe,
	}
}

// GenerateConfig generates NATS configuration from PocketBase data
func (g *Generator) GenerateConfig(roles []models.MqttRole, users []models.MqttUser) (string, error) {
	// Create role map for easy lookup
	roleMap := make(map[string]models.MqttRole)
	for _, role := range roles {
		roleMap[role.ID] = role
	}

	// Format default permissions
	defaultPublishStr, defaultSubscribeStr := models.FormatDefaultPermissions(g.defaultPublish, g.defaultSubscribe)

	// Create data for NATS config template
	configData := &models.NatsConfigData{
		DefaultPublish:  defaultPublishStr,
		DefaultSubscribe: defaultSubscribeStr,
		Roles:           []models.NatsRole{},
		Users:           []models.NatsUser{},
	}
	
	// Log permissions parsing
	g.logger.Debug("Parsing role permissions from JSON fields")

	// Add roles
	for _, role := range roles {
		// Format permissions with error handling
		pubPerms := role.FormatPublishPermissions()
		subPerms := role.FormatSubscribePermissions()
		
		g.logger.Debug("Formatted role permissions",
			zap.String("role", role.Name),
			zap.String("publish", pubPerms),
			zap.String("subscribe", subPerms))
		
		configData.Roles = append(configData.Roles, models.NatsRole{
			Name:                role.NormalizeRoleName(),
			PublishPermissions:  pubPerms,
			SubscribePermissions: subPerms,
		})
	}

	// Add users
	for i, user := range users {
		// Find the role for this user
		role, ok := roleMap[user.RoleID]
		if !ok {
			g.logger.Warn("User has unknown role ID, skipping", 
				zap.String("username", user.Username), 
				zap.String("role_id", user.RoleID))
			continue
		}

		// Add user to config
		configData.Users = append(configData.Users, models.NatsUser{
			Username: fmt.Sprintf("\"%s\"", user.Username),
			Password: user.Password,
			RoleName: role.NormalizeRoleName(),
			IsLast:   i == len(users)-1,
		})
	}
	
	// Sort roles by name for deterministic output
	sort.Slice(configData.Roles, func(i, j int) bool {
		return configData.Roles[i].Name < configData.Roles[j].Name
	})

	// Sort users by username for deterministic output
	sort.Slice(configData.Users, func(i, j int) bool {
		return strings.ToLower(configData.Users[i].Username) < strings.ToLower(configData.Users[j].Username)
	})

	// Update IsLast flag based on new order
	for i := range configData.Users {
		configData.Users[i].IsLast = (i == len(configData.Users)-1)
	}

	// Generate the NATS config
	config, err := models.FormatConfigFile(configData)
	if err != nil {
		return "", fmt.Errorf("failed to format NATS config: %w", err)
	}

	g.logger.Info("Generated NATS configuration",
		zap.Int("roleCount", len(configData.Roles)),
		zap.Int("userCount", len(configData.Users)))

	return config, nil
}
