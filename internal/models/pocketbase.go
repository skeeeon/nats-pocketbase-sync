package models

import (
	"encoding/json"
	"strings"
	"time"
)

// MqttUser represents a user in the PocketBase MQTT users collection
type MqttUser struct {
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	Password   string    `json:"password"`
	RoleID     string    `json:"role_id"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created"`
	UpdatedAt  time.Time `json:"updated"`
}

// MqttRole represents a role in the PocketBase MQTT roles collection
type MqttRole struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	PublishPermissions  json.RawMessage `json:"publish_permissions"`
	SubscribePermissions json.RawMessage `json:"subscribe_permissions"`
	CreatedAt           time.Time       `json:"created"`
	UpdatedAt           time.Time       `json:"updated"`
}

// PocketBaseListResponse represents a generic list response from PocketBase
type PocketBaseListResponse[T any] struct {
	Page       int    `json:"page"`
	PerPage    int    `json:"perPage"`
	TotalItems int    `json:"totalItems"`
	TotalPages int    `json:"totalPages"`
	Items      []T    `json:"items"`
}

// PocketBaseResponse represents a generic single item response from PocketBase
type PocketBaseResponse[T any] struct {
	Item T `json:"item"`
}

// PocketBaseAuthResponse represents an authentication response from PocketBase
type PocketBaseAuthResponse struct {
	Token  string      `json:"token"`
	Record interface{} `json:"record"`
}

// NormalizeRoleName ensures the role name is valid for NATS config
func (r *MqttRole) NormalizeRoleName() string {
	// Convert to uppercase and replace spaces/special chars with underscores
	name := strings.ToUpper(r.Name)
	name = strings.ReplaceAll(name, " ", "_")
	
	// Remove any characters that aren't alphanumeric or underscore
	var result strings.Builder
	for _, char := range name {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' {
			result.WriteRune(char)
		}
	}
	
	return result.String()
}

// GetPublishPermissions extracts the string array from JSON field
func (r *MqttRole) GetPublishPermissions() ([]string, error) {
	var permissions []string
	if len(r.PublishPermissions) == 0 {
		return permissions, nil
	}
	
	if err := json.Unmarshal(r.PublishPermissions, &permissions); err != nil {
		return nil, err
	}
	return permissions, nil
}

// GetSubscribePermissions extracts the string array from JSON field
func (r *MqttRole) GetSubscribePermissions() ([]string, error) {
	var permissions []string
	if len(r.SubscribePermissions) == 0 {
		return permissions, nil
	}
	
	if err := json.Unmarshal(r.SubscribePermissions, &permissions); err != nil {
		return nil, err
	}
	return permissions, nil
}

// FormatPublishPermissions formats the publish permissions for NATS config
func (r *MqttRole) FormatPublishPermissions() string {
	permissions, err := r.GetPublishPermissions()
	if err != nil {
		// In case of error, return empty string as default
		return `""`
	}
	
	if len(permissions) == 0 {
		return `""`
	}
	
	if len(permissions) == 1 {
		return `"` + permissions[0] + `"`
	}
	
	var result strings.Builder
	result.WriteString("[")
	for i, perm := range permissions {
		result.WriteString(`"` + perm + `"`)
		if i < len(permissions)-1 {
			result.WriteString(", ")
		}
	}
	result.WriteString("]")
	
	return result.String()
}

// FormatSubscribePermissions formats the subscribe permissions for NATS config
func (r *MqttRole) FormatSubscribePermissions() string {
	permissions, err := r.GetSubscribePermissions()
	if err != nil {
		// In case of error, return empty string as default
		return `""`
	}
	
	if len(permissions) == 0 {
		return `""`
	}
	
	if len(permissions) == 1 {
		return `"` + permissions[0] + `"`
	}
	
	var result strings.Builder
	result.WriteString("[")
	for i, perm := range permissions {
		result.WriteString(`"` + perm + `"`)
		if i < len(permissions)-1 {
			result.WriteString(", ")
		}
	}
	result.WriteString("]")
	
	return result.String()
}
