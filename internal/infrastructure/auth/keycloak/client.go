package keycloak

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// AuthProvider defines the interface for authentication operations.
type AuthProvider interface {
	VerifyToken(ctx context.Context, rawToken string) (*TokenClaims, error)
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
	IntrospectToken(ctx context.Context, token string) (*IntrospectionResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	GetServiceToken(ctx context.Context) (string, error)
	Logout(ctx context.Context, refreshToken string) error
	Health(ctx context.Context) error
}

// TokenClaims represents the claims parsed from a JWT access token.
type TokenClaims struct {
	Subject           string              `json:"sub"`
	Email             string              `json:"email"`
	PreferredUsername string              `json:"preferred_username"`
	RealmRoles        []string            `json:"realm_roles"`
	ClientRoles       map[string][]string `json:"client_roles"`
	Groups            []string            `json:"groups"`
	TenantID          string              `json:"tenant_id"`
	IssuedAt          time.Time           `json:"iat"`
	ExpiresAt         time.Time           `json:"exp"`
	Issuer            string              `json:"iss"`
	Audience          []string            `json:"aud"`
	Scope             string              `json:"scope"`
}

// UserInfo represents user information returned by the UserInfo endpoint.
type UserInfo struct {
	ID                string              `json:"sub"`
	Email             string              `json:"email"`
	EmailVerified     bool                `json:"email_verified"`
	Name              string              `json:"name"`
	PreferredUsername string              `json:"preferred_username"`
	GivenName         string              `json:"given_name"`
	FamilyName        string              `json:"family_name"`
	Roles             []string            `json:"roles"`
	Groups            []string            `json:"groups"`
	TenantID          string              `json:"tenant_id"`
	Attributes        map[string][]string `json:"attributes"`
}

// IntrospectionResult represents the result of token introspection.
type IntrospectionResult struct {
	Active    bool      `json:"active"`
	Subject   string    `json:"sub"`
	ClientID  string    `json:"client_id"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"exp"`
	Scope     string    `json:"scope"`
	Roles     []string  `json:"roles"`
}

// TokenPair represents a pair of access and refresh tokens.
type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
}

// KeycloakConfig holds configuration for the Keycloak client.
type KeycloakConfig struct {
	BaseURL                  string        `json:"base_url"`
	Realm                    string        `json:"realm"`
	ClientID                 string        `json:"client_id"`
	ClientSecret             string        `json:"client_secret"`
	PublicKeyRefreshInterval time.Duration `json:"public_key_refresh_interval"`
	RequestTimeout           time.Duration `json:"request_timeout"`
	RetryAttempts            int           `json:"retry_attempts"`
	RetryDelay               time.Duration `json:"retry_delay"`
	TLSInsecureSkipVerify    bool          `json:"tls_insecure_skip_verify"`
}

// keycloakClient implements the AuthProvider interface.
type keycloakClient struct {
	config            KeycloakConfig
	httpClient        *http.Client
	jwksCache         *jwksCache
	serviceTokenCache *serviceTokenEntry
	logger            logging.Logger
	metrics           MetricsCollector
}

// MetricsCollector is an interface for metrics collection.
type MetricsCollector interface {
	// Add metric collection methods here if needed
}

// NewKeycloakClient creates a new instance of keycloakClient.
func NewKeycloakClient(cfg KeycloakConfig, logger logging.Logger, opts ...ClientOption) (AuthProvider, error) {
	if cfg.BaseURL == "" {
		return nil, ErrInvalidConfig.WithInternalMessage("BaseURL is required")
	}
	if cfg.Realm == "" {
		return nil, ErrInvalidConfig.WithInternalMessage("Realm is required")
	}
	if cfg.ClientID == "" {
		return nil, ErrInvalidConfig.WithInternalMessage("ClientID is required")
	}

	// Normalize BaseURL
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	if cfg.PublicKeyRefreshInterval == 0 {
		cfg.PublicKeyRefreshInterval = 5 * time.Minute
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
	if cfg.RetryAttempts == 0 {
		cfg.RetryAttempts = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 500 * time.Millisecond
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
		},
	}

	client := &keycloakClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: transport,
		},
		jwksCache:         newJWKSCache(),
		serviceTokenCache: &serviceTokenEntry{},
		logger:            logger,
	}

	for _, opt := range opts {
		opt(client)
	}

	// Initial JWKS fetch
	if err := client.refreshJWKS(context.Background()); err != nil {
		return nil, ErrJWKSRefreshFailed.WithCause(err)
	}

	// Start background refresh
	go client.startJWKSRefresh()

	return client, nil
}

func (c *keycloakClient) startJWKSRefresh() {
	ticker := time.NewTicker(c.config.PublicKeyRefreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := c.refreshJWKS(context.Background()); err != nil {
			c.logger.Error("failed to refresh JWKS", logging.Error(err))
		}
	}
}

func (c *keycloakClient) refreshJWKS(ctx context.Context) error {
	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", c.config.BaseURL, c.config.Realm)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch JWKS: status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, raw := range jwks.Keys {
		var keyData struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		}
		if err := json.Unmarshal(raw, &keyData); err != nil {
			continue
		}

		if keyData.Kty != "RSA" {
			continue
		}

		pubKey, err := parseRSAPublicKey(keyData.N, keyData.E)
		if err != nil {
			c.logger.Warn("failed to parse RSA key", logging.String("kid", keyData.Kid), logging.Error(err))
			continue
		}
		keys[keyData.Kid] = pubKey
	}

	c.jwksCache.update(keys)
	return nil
}

// doRequest executes HTTP request with retry logic for 5xx errors.
func (c *keycloakClient) doRequest(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i <= c.config.RetryAttempts; i++ {
		if i > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(c.config.RetryDelay * time.Duration(1<<uint(i-1))): // Exponential backoff
			}
		}

		// Clone request body if needed? No, usually body is stream.
		// If body is read once, we can't retry unless we buffer it.
		// For GET requests, body is nil.
		// For POST requests, we assume body reader supports seeking or is bytes buffer.
		// Standard `http.NewRequest` wraps strings/bytes readers which support seek.
		// If generic reader, we might have issues.
		// However, in this client, we use `strings.NewReader` for POSTs.
		if req.Body != nil {
			if seeker, ok := req.Body.(io.Seeker); ok {
				seeker.Seek(0, io.SeekStart)
			} else if req.GetBody != nil {
				// Use GetBody to refresh body
				newBody, err := req.GetBody()
				if err == nil {
					req.Body = newBody
				}
			}
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			// Network error, retry
			c.logger.Warn("request failed, retrying", logging.Error(err), logging.Int("attempt", i+1))
			continue
		}

		if resp.StatusCode >= 500 {
			resp.Body.Close()
			c.logger.Warn("server error, retrying", logging.Int("status", resp.StatusCode), logging.Int("attempt", i+1))
			continue
		}

		// Success or 4xx error (no retry)
		return resp, nil
	}

	if err != nil {
		return nil, err
	}
	return nil, ErrKeycloakUnavailable.WithInternalMessage("max retries exceeded")
}

// VerifyToken validates the token and returns the claims.
func (c *keycloakClient) VerifyToken(ctx context.Context, rawToken string) (*TokenClaims, error) {
	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("kid missing from header")
		}

		pubKey, ok := c.jwksCache.getKey(kid)
		if !ok {
			// Try refreshing JWKS once
			if err := c.refreshJWKS(ctx); err != nil {
				return nil, fmt.Errorf("key not found and refresh failed")
			}
			pubKey, ok = c.jwksCache.getKey(kid)
			if !ok {
				return nil, fmt.Errorf("key not found")
			}
		}
		return pubKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, ErrTokenInvalidSignature
		}
		return nil, ErrTokenMalformed.WithCause(err)
	}

	if !token.Valid {
		return nil, ErrTokenInvalidSignature
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenMalformed.WithInternalMessage("invalid claims type")
	}

	// Validate Issuer
	expectedIssuer := fmt.Sprintf("%s/realms/%s", c.config.BaseURL, c.config.Realm)
	iss, _ := claims.GetIssuer()
	if iss != expectedIssuer {
		return nil, ErrTokenInvalidIssuer.WithDetail(fmt.Sprintf("expected %s, got %s", expectedIssuer, iss))
	}

	// Validate Audience
	aud, _ := claims.GetAudience()
	audFound := false
	for _, a := range aud {
		if a == c.config.ClientID {
			audFound = true
			break
		}
	}
	if !audFound {
		return nil, ErrTokenInvalidAudience.WithDetail(fmt.Sprintf("expected %s", c.config.ClientID))
	}

	// Map claims to TokenClaims
	tc := &TokenClaims{
		Subject:           mustGetString(claims, "sub"),
		Email:             getString(claims, "email"),
		PreferredUsername: getString(claims, "preferred_username"),
		Issuer:            iss,
		Audience:          aud,
		Scope:             getString(claims, "scope"),
	}

	if iat, err := claims.GetIssuedAt(); err == nil && iat != nil {
		tc.IssuedAt = iat.Time
	}
	if exp, err := claims.GetExpirationTime(); err == nil && exp != nil {
		tc.ExpiresAt = exp.Time
	}

	// Extract Realm Roles
	if realmAccess, ok := claims["realm_access"].(map[string]interface{}); ok {
		if roles, ok := realmAccess["roles"].([]interface{}); ok {
			for _, r := range roles {
				if rStr, ok := r.(string); ok {
					tc.RealmRoles = append(tc.RealmRoles, rStr)
				}
			}
		}
	}

	// Extract Client Roles
	if resourceAccess, ok := claims["resource_access"].(map[string]interface{}); ok {
		tc.ClientRoles = make(map[string][]string)
		for clientID, access := range resourceAccess {
			if accessMap, ok := access.(map[string]interface{}); ok {
				if roles, ok := accessMap["roles"].([]interface{}); ok {
					var clientRoleList []string
					for _, r := range roles {
						if rStr, ok := r.(string); ok {
							clientRoleList = append(clientRoleList, rStr)
						}
					}
					tc.ClientRoles[clientID] = clientRoleList
				}
			}
		}
	}

	// Extract TenantID
	tc.TenantID = getString(claims, "tenant_id")

	return tc, nil
}

func (c *keycloakClient) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/userinfo", c.config.BaseURL, c.config.Realm)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrTokenExpired
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrKeycloakUnavailable.WithInternalMessage(fmt.Sprintf("userinfo status: %d", resp.StatusCode))
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}
	return &userInfo, nil
}

func (c *keycloakClient) IntrospectToken(ctx context.Context, token string) (*IntrospectionResult, error) {
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token/introspect", c.config.BaseURL, c.config.Realm)
	data := url.Values{}
	data.Set("token", token)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrTokenIntrospectionFailed.WithInternalMessage(fmt.Sprintf("status: %d", resp.StatusCode))
	}

	var result IntrospectionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *keycloakClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.config.BaseURL, c.config.Realm)
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, ErrTokenExpired.WithInternalMessage(fmt.Sprintf("refresh failed: %s", string(body)))
	}

	var pair TokenPair
	if err := json.NewDecoder(resp.Body).Decode(&pair); err != nil {
		return nil, err
	}
	return &pair, nil
}

func (c *keycloakClient) GetServiceToken(ctx context.Context) (string, error) {
	c.serviceTokenCache.mu.RLock()
	if c.serviceTokenCache.isValid() {
		token := c.serviceTokenCache.token
		c.serviceTokenCache.mu.RUnlock()
		return token, nil
	}
	c.serviceTokenCache.mu.RUnlock()

	c.serviceTokenCache.mu.Lock()
	defer c.serviceTokenCache.mu.Unlock()

	// Double check
	if c.serviceTokenCache.isValid() {
		return c.serviceTokenCache.token, nil
	}

	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.config.BaseURL, c.config.Realm)
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ErrKeycloakUnavailable.WithInternalMessage("failed to get service token")
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	c.serviceTokenCache.token = result.AccessToken
	c.serviceTokenCache.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return result.AccessToken, nil
}

func (c *keycloakClient) Logout(ctx context.Context, refreshToken string) error {
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout", c.config.BaseURL, c.config.Realm)
	data := url.Values{}
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("logout failed with status %d", resp.StatusCode)
	}
	return nil
}

func (c *keycloakClient) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/realms/%s/.well-known/openid-configuration", c.config.BaseURL, c.config.Realm)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return ErrKeycloakUnavailable.WithCause(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ErrKeycloakUnavailable
	}
	return nil
}

// Internal structures

type jwksCache struct {
	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
}

func newJWKSCache() *jwksCache {
	return &jwksCache{
		keys: make(map[string]*rsa.PublicKey),
	}
}

func (c *jwksCache) update(keys map[string]*rsa.PublicKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = keys
}

func (c *jwksCache) getKey(kid string) (*rsa.PublicKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	k, ok := c.keys[kid]
	return k, ok
}

type serviceTokenEntry struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
}

func (e *serviceTokenEntry) isValid() bool {
	return time.Now().Add(30 * time.Second).Before(e.expiresAt)
}

// ClientOption defines functional options for configuration.
type ClientOption func(*keycloakClient)

func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *keycloakClient) {
		c.httpClient = client
	}
}

func WithMetrics(collector MetricsCollector) ClientOption {
	return func(c *keycloakClient) {
		c.metrics = collector
	}
}

func WithJWKSRefreshInterval(d time.Duration) ClientOption {
	return func(c *keycloakClient) {
		c.config.PublicKeyRefreshInterval = d
	}
}

// Errors
var (
	ErrTokenExpired             = errors.ErrUnauthorized("token expired")
	ErrTokenInvalidSignature    = errors.ErrUnauthorized("invalid token signature")
	ErrTokenInvalidIssuer       = errors.ErrUnauthorized("invalid token issuer")
	ErrTokenInvalidAudience     = errors.ErrUnauthorized("invalid token audience")
	ErrTokenMalformed           = errors.ErrUnauthorized("malformed token")
	ErrTokenIntrospectionFailed = errors.ErrInternal("token introspection failed")
	ErrKeycloakUnavailable      = errors.ErrServiceUnavailable("Keycloak")
	ErrJWKSRefreshFailed        = errors.ErrInternal("failed to refresh JWKS")
	ErrInvalidConfig            = errors.ErrInternal("invalid configuration")
)

// Helpers
func getString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

func mustGetString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

// parseRSAPublicKey parses n and e strings (base64url encoded) into an RSA Public Key.
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	// Base64 URL decode
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode e: %w", err)
	}

	var eInt int
	if len(eBytes) <= 8 { // Allow up to 64-bit e, typically 3 or 65537
		for _, b := range eBytes {
			eInt = (eInt << 8) | int(b)
		}
	} else {
		return nil, fmt.Errorf("exponent too large")
	}

	pub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eInt,
	}
	return pub, nil
}
//Personal.AI order the ending
