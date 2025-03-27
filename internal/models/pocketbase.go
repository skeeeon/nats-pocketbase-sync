package models

import (
	"encoding/json"
	"strings"
	"time"
)

// FlexibleTime is a custom time type that can handle various timestamp formats
// including empty strings and space-delimited timestamps
type FlexibleTime time.Time

// UnmarshalJSON custom unmarshaler for handling various time formats from PocketBase
func (ft *FlexibleTime) UnmarshalJSON(data []byte) error {
	s := string(data)
	
	// Handle empty or null values
	if s == "\"\"" || s == "null" {
		*ft = FlexibleTime(time.Time{})
		return nil
	}
	
	// Remove quotes
	s = strings.Trim(s, "\"")
	
	// Try standard RFC3339 format first
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		*ft = FlexibleTime(t)
		return nil
	}
	
	// Try space-delimited format with Z timezone
	t, err = time.Parse("2006-01-02 15:04:05.999Z", s)
	if err == nil {
		*ft = FlexibleTime(t)
		return nil
	}
	
	// Try space-delimited format without timezone
	t, err = time.Parse("2006-01-02 15:04:05.999", s)
	if err == nil {
		*ft = FlexibleTime(t)
		return nil
	}
	
	// Try space-delimited format with seconds precision
	t, err = time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		*ft = FlexibleTime(t)
		return nil
	}
	
	// Try date-only format
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		*ft = FlexibleTime(t)
		return nil
	}
	
	// If all parsing attempts fail, return the last error
	return err
}

// Time returns the underlying time.Time value
func (ft FlexibleTime) Time() time.Time {
	return time.Time(ft)
}

// MqttUser represents a user in the PocketBase MQTT users collection
type MqttUser struct {
	ID              string        `json:"id"`
	Username        string        `json:"username"`
	Password        string        `json:"password"`
	RoleID          string        `json:"role_id"`
	Active          bool          `json:"active"`
	CollectionID    string        `json:"collectionId,omitempty"`
	CollectionName  string        `json:"collectionName,omitempty"`
	Created         FlexibleTime  `json:"created"`
	Updated         FlexibleTime  `json:"updated"`
}

// MqttRole represents a role in the PocketBase MQTT roles collection
type MqttRole struct {
	ID                   string        `json:"id"`
	Name                 string        `json:"name"`
	PublishPermissions   json.RawMessage `json:"publish_permissions"`
	SubscribePermissions json.RawMessage `json:"subscribe_permissions"`
	CollectionID         string        `json:"collectionId,omitempty"`
	CollectionName       string        `json:"collectionName,omitempty"`
	Created              FlexibleTime  `json:"created"`
	Updated              FlexibleTime  `json:"updated"`
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
