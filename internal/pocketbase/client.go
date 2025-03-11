package pocketbase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"nats-pocketbase-sync/internal/models"
	"go.uber.org/zap"
)

// min returns the smaller of x or y.
func min(x, y int) int {
	return int(math.Min(float64(x), float64(y)))
}

// Client is a PocketBase API client
type Client struct {
	baseURL     string
	httpClient  *http.Client
	authToken   string
	logger      *zap.Logger
	collections struct {
		users string
		roles string
	}
}

// NewClient creates a new PocketBase client
func NewClient(baseURL, userCollection, roleCollection string, logger *zap.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
		collections: struct {
			users string
			roles string
		}{
			users: userCollection,
			roles: roleCollection,
		},
	}
}

// Authenticate authenticates with PocketBase using credentials
func (c *Client) Authenticate(email, password string) error {
	data := map[string]string{
		"identity":    email,    // PocketBase uses "identity" for username/email
		"password": password,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}

	// Use the correct authentication endpoint for collections
	authEndpoint := fmt.Sprintf("%s/api/collections/_superusers/auth-with-password", c.baseURL)
	c.logger.Debug("Authenticating with PocketBase", zap.String("endpoint", authEndpoint))

	req, err := http.NewRequest("POST", authEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp models.PocketBaseAuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	c.authToken = authResp.Token
	c.logger.Info("Successfully authenticated with PocketBase")
	return nil
}

// GetAllMqttUsers retrieves all MQTT users from PocketBase
func (c *Client) GetAllMqttUsers() ([]models.MqttUser, error) {
	if c.authToken == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	// Construct URL with filters for active users
	endpoint := fmt.Sprintf("%s/api/collections/%s/records", c.baseURL, c.collections.users)
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	query := reqURL.Query()
	query.Set("filter", "active=true")
	query.Set("perPage", "100") // Adjust based on expected user count
	reqURL.RawQuery = query.Encode()

	c.logger.Debug("Fetching MQTT users", 
		zap.String("url", reqURL.String()),
		zap.String("auth_token_prefix", c.authToken[:10]+"...")) // Log only prefix for security

	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create users request: %w", err)
	}

	// Create a consistent output format
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send users request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("users request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var usersResp models.PocketBaseListResponse[models.MqttUser]
	if err := json.Unmarshal(body, &usersResp); err != nil {
		return nil, fmt.Errorf("failed to decode users response: %w", err)
	}

	c.logger.Info("Retrieved MQTT users from PocketBase", zap.Int("count", len(usersResp.Items)))
	return usersResp.Items, nil
}

// GetAllMqttRoles retrieves all MQTT roles from PocketBase
func (c *Client) GetAllMqttRoles() ([]models.MqttRole, error) {
	if c.authToken == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	endpoint := fmt.Sprintf("%s/api/collections/%s/records", c.baseURL, c.collections.roles)
	c.logger.Debug("Fetching MQTT roles", zap.String("url", endpoint))
	
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create roles request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send roles request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("roles request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rolesResp models.PocketBaseListResponse[models.MqttRole]
	if err := json.Unmarshal(body, &rolesResp); err != nil {
		c.logger.Error("Failed to decode roles response", 
			zap.Error(err), 
			zap.String("response", string(body[:min(len(body), 1000)]))) // Log first 1000 chars
		return nil, fmt.Errorf("failed to decode roles response: %w", err)
	}

	c.logger.Info("Retrieved MQTT roles from PocketBase", zap.Int("count", len(rolesResp.Items)))
	return rolesResp.Items, nil
}

// GetRoleByID retrieves a specific role by ID
func (c *Client) GetRoleByID(roleID string) (*models.MqttRole, error) {
	if c.authToken == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	endpoint := fmt.Sprintf("%s/api/collections/%s/records/%s", c.baseURL, c.collections.roles, roleID)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create role request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send role request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("role request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var roleResp models.PocketBaseResponse[models.MqttRole]
	if err := json.NewDecoder(resp.Body).Decode(&roleResp); err != nil {
		return nil, fmt.Errorf("failed to decode role response: %w", err)
	}

	return &roleResp.Item, nil
}
