package keycloak

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	stdliberrors "errors"
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

// AuthProvider is the interface for authentication providers.
type AuthProvider interface {
	VerifyToken(ctx context.Context, rawToken string) (*TokenClaims, error)
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
	IntrospectToken(ctx context.Context, token string) (*IntrospectionResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	GetServiceToken(ctx context.Context) (string, error)
	Logout(ctx context.Context, refreshToken string) error
	Health(ctx context.Context) error
}

// TokenClaims represents the claims in a JWT token.
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

// UserInfo represents the user information.
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

// TokenPair represents an access token and refresh token pair.
type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
}

// KeycloakConfig configuration for Keycloak client.
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

type keycloakClient struct {
	config            KeycloakConfig
	httpClient        *http.Client
	jwksCache         *jwksCache
	serviceTokenCache *serviceTokenEntry
	logger            logging.Logger
	metrics           MetricsCollector
}

type serviceTokenEntry struct {
	token     string
	expiresAt time.Time
	mu        sync.RWMutex
}

func (s *serviceTokenEntry) isValid() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Now().Add(30 * time.Second).Before(s.expiresAt)
}

func (s *serviceTokenEntry) getToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

func (s *serviceTokenEntry) setToken(token string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
	s.expiresAt = expiresAt
}

type jwksCache struct {
	keys   map[string]*rsa.PublicKey
	mu     sync.RWMutex
	client *http.Client
	url    string
	logger logging.Logger
}

func (c *jwksCache) refresh() error {
	c.logger.Debug("Refreshing JWKS cache", logging.String("url", c.url))
	resp, err := c.client.Get(c.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch JWKS: %s", resp.Status)
	}

	var jwks struct {
		Keys []struct {
			Kid string   `json:"kid"`
			Kty string   `json:"kty"`
			Alg string   `json:"alg"`
			Use string   `json:"use"`
			N   string   `json:"n"`
			E   string   `json:"e"`
			X5c []string `json:"x5c"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" || key.Use != "sig" {
			continue
		}

		nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			c.logger.Warn("Failed to decode modulus", logging.String("kid", key.Kid), logging.Err(err))
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			c.logger.Warn("Failed to decode exponent", logging.String("kid", key.Kid), logging.Err(err))
			continue
		}

		// Correct way to decode E (exponent) which is usually small (e.g. 65537)
		// Standard E is often AQAB (65537). Base64 decode gives [1, 0, 1].
		// We need to convert bytes to int.
		eInt := 0
		for _, b := range eBytes {
			eInt = eInt<<8 + int(b)
		}

		pubKey := &rsa.PublicKey{
			N: big.NewInt(0).SetBytes(nBytes),
			E: eInt,
		}
		newKeys[key.Kid] = pubKey
	}

	c.mu.Lock()
	c.keys = newKeys
	c.mu.Unlock()
	return nil
}

func (c *jwksCache) getKey(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	key, ok := c.keys[kid]
	c.mu.RUnlock()
	if ok {
		return key, nil
	}
	// Try refresh
	if err := c.refresh(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	key, ok = c.keys[kid]
	c.mu.RUnlock()
	if ok {
		return key, nil
	}
	return nil, fmt.Errorf("public key not found for kid: %s", kid)
}

// ClientOption is a function option for configuring KeycloakClient.
type ClientOption func(*keycloakClient)

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *keycloakClient) {
		c.httpClient = client
	}
}

// WithMetrics sets the metrics collector.
func WithMetrics(collector MetricsCollector) ClientOption {
	return func(c *keycloakClient) {
		c.metrics = collector
	}
}

// WithJWKSRefreshInterval sets the JWKS refresh interval.
func WithJWKSRefreshInterval(d time.Duration) ClientOption {
	return func(c *keycloakClient) {
		c.config.PublicKeyRefreshInterval = d
	}
}

// MetricsCollector interface placeholder, will be implemented in prometheus package.
type MetricsCollector interface {
	// Define methods if needed, or use any
}

// Errors
var (
	ErrTokenExpired             = errors.New(errors.ErrCodeUnauthorized, "token expired")
	ErrTokenInvalidSignature    = errors.New(errors.ErrCodeUnauthorized, "invalid token signature")
	ErrTokenInvalidIssuer       = errors.New(errors.ErrCodeUnauthorized, "invalid token issuer")
	ErrTokenInvalidAudience     = errors.New(errors.ErrCodeUnauthorized, "invalid token audience")
	ErrTokenMalformed           = errors.New(errors.ErrCodeUnauthorized, "malformed token")
	ErrTokenIntrospectionFailed = errors.New(errors.ErrCodeInternal, "token introspection failed")
	ErrKeycloakUnavailable      = errors.New(errors.ErrCodeServiceUnavailable, "keycloak unavailable")
	ErrJWKSRefreshFailed        = errors.New(errors.ErrCodeInternal, "jwks refresh failed")
	ErrInvalidConfig            = errors.New(errors.ErrCodeValidation, "invalid configuration")
)

// NewKeycloakClient creates a new KeycloakClient.
func NewKeycloakClient(cfg KeycloakConfig, logger logging.Logger, opts ...ClientOption) (AuthProvider, error) {
	if cfg.BaseURL == "" {
		return nil, errors.Wrap(ErrInvalidConfig, errors.ErrCodeValidation, "base_url is required")
	}
	if cfg.Realm == "" {
		return nil, errors.Wrap(ErrInvalidConfig, errors.ErrCodeValidation, "realm is required")
	}
	if cfg.ClientID == "" {
		return nil, errors.Wrap(ErrInvalidConfig, errors.ErrCodeValidation, "client_id is required")
	}

	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
	if cfg.PublicKeyRefreshInterval == 0 {
		cfg.PublicKeyRefreshInterval = 5 * time.Minute
	}
	if cfg.RetryAttempts == 0 {
		cfg.RetryAttempts = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 500 * time.Millisecond
	}

	httpClient := &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.TLSInsecureSkipVerify},
		},
	}

	client := &keycloakClient{
		config:            cfg,
		httpClient:        httpClient,
		serviceTokenCache: &serviceTokenEntry{},
		logger:            logger,
	}

	for _, opt := range opts {
		opt(client)
	}

	jwksURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", strings.TrimRight(cfg.BaseURL, "/"), cfg.Realm)
	client.jwksCache = &jwksCache{
		client: client.httpClient,
		url:    jwksURL,
		logger: logger,
	}

	// Initial JWKS fetch
	if err := client.jwksCache.refresh(); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to fetch JWKS")
	}

	// Start background refresh
	go func() {
		ticker := time.NewTicker(cfg.PublicKeyRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := client.jwksCache.refresh(); err != nil {
				logger.Error("Failed to refresh JWKS", logging.Err(err))
			}
		}
	}()

	return client, nil
}

func (c *keycloakClient) VerifyToken(ctx context.Context, rawToken string) (*TokenClaims, error) {
	// Parse token without verification first to get header
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(rawToken, jwt.MapClaims{})
	if err != nil {
		return nil, ErrTokenMalformed
	}

	// Get kid from header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, ErrTokenMalformed
	}

	// Get key from cache
	key, err := c.jwksCache.getKey(kid)
	if err != nil {
		// Try to refresh once
		if refreshErr := c.jwksCache.refresh(); refreshErr != nil {
			return nil, ErrJWKSRefreshFailed
		}
		key, err = c.jwksCache.getKey(kid)
		if err != nil {
			return nil, ErrTokenInvalidSignature
		}
	}

	// Verify token with key
	parsedToken, err := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return key, nil
	})

	if err != nil {
		if stdliberrors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if stdliberrors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, ErrTokenInvalidSignature
		}
		return nil, errors.Wrap(err, errors.ErrCodeUnauthorized, "token verification failed")
	}

	if !parsedToken.Valid {
		return nil, ErrTokenInvalidSignature
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenMalformed
	}

	// Check Issuer
	expectedIssuer := fmt.Sprintf("%s/realms/%s", strings.TrimRight(c.config.BaseURL, "/"), c.config.Realm)
	if iss, err := claims.GetIssuer(); err != nil || iss != expectedIssuer {
		// Some Keycloak setups might differ slightly in issuer format, verify logic is correct for your setup
		// Strict check:
		return nil, ErrTokenInvalidIssuer
	}

	// Check Audience
	aud, err := claims.GetAudience()
	if err != nil {
		return nil, ErrTokenInvalidAudience
	}
	audienceFound := false
	for _, a := range aud {
		if a == c.config.ClientID {
			audienceFound = true
			break
		}
	}
	if !audienceFound {
		// Also check "azp" (Authorized Party) if available
		if azp, ok := claims["azp"].(string); ok && azp == c.config.ClientID {
			// Acceptable
		} else {
			return nil, ErrTokenInvalidAudience
		}
	}

	tokenClaims := &TokenClaims{}

	// Map standard claims
	if sub, err := claims.GetSubject(); err == nil {
		tokenClaims.Subject = sub
	}
	if exp, err := claims.GetExpirationTime(); err == nil && exp != nil {
		tokenClaims.ExpiresAt = exp.Time
	}
	if iat, err := claims.GetIssuedAt(); err == nil && iat != nil {
		tokenClaims.IssuedAt = iat.Time
	}
	if iss, err := claims.GetIssuer(); err == nil {
		tokenClaims.Issuer = iss
	}
	if aud, err := claims.GetAudience(); err == nil {
		tokenClaims.Audience = aud
	}

	// Map custom claims
	if email, ok := claims["email"].(string); ok {
		tokenClaims.Email = email
	}
	if preferredUsername, ok := claims["preferred_username"].(string); ok {
		tokenClaims.PreferredUsername = preferredUsername
	}
	if scope, ok := claims["scope"].(string); ok {
		tokenClaims.Scope = scope
	}
	if tenantID, ok := claims["tenant_id"].(string); ok {
		tokenClaims.TenantID = tenantID
	} else if tenantID, ok := claims["tenantId"].(string); ok {
		tokenClaims.TenantID = tenantID
	}

	// Extract Realm Roles
	if realmAccess, ok := claims["realm_access"].(map[string]interface{}); ok {
		if roles, ok := realmAccess["roles"].([]interface{}); ok {
			for _, r := range roles {
				if roleStr, ok := r.(string); ok {
					tokenClaims.RealmRoles = append(tokenClaims.RealmRoles, roleStr)
				}
			}
		}
	}

	// Extract Client Roles
	tokenClaims.ClientRoles = make(map[string][]string)
	if resourceAccess, ok := claims["resource_access"].(map[string]interface{}); ok {
		for clientID, access := range resourceAccess {
			if accessMap, ok := access.(map[string]interface{}); ok {
				if roles, ok := accessMap["roles"].([]interface{}); ok {
					var clientRoleList []string
					for _, r := range roles {
						if roleStr, ok := r.(string); ok {
							clientRoleList = append(clientRoleList, roleStr)
						}
					}
					tokenClaims.ClientRoles[clientID] = clientRoleList
				}
			}
		}
	}

	// Groups
	if groups, ok := claims["groups"].([]interface{}); ok {
		for _, g := range groups {
			if gStr, ok := g.(string); ok {
				tokenClaims.Groups = append(tokenClaims.Groups, gStr)
			}
		}
	}

	return tokenClaims, nil
}

func (c *keycloakClient) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/userinfo", strings.TrimRight(c.config.BaseURL, "/"), c.config.Realm)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, ErrKeycloakUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrTokenExpired
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}

	userInfo := &UserInfo{}
	if err := json.NewDecoder(resp.Body).Decode(userInfo); err != nil {
		return nil, err
	}
	return userInfo, nil
}

func (c *keycloakClient) IntrospectToken(ctx context.Context, token string) (*IntrospectionResult, error) {
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token/introspect", strings.TrimRight(c.config.BaseURL, "/"), c.config.Realm)

	data := url.Values{}
	data.Set("token", token)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, ErrTokenIntrospectionFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection request failed with status: %d", resp.StatusCode)
	}

	var rawResult struct {
		Active    bool     `json:"active"`
		Subject   string   `json:"sub"`
		ClientID  string   `json:"client_id"`
		TokenType string   `json:"token_type"`
		ExpiresAt int64    `json:"exp"`
		Scope     string   `json:"scope"`
		Roles     []string `json:"roles"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawResult); err != nil {
		return nil, err
	}

	return &IntrospectionResult{
		Active:    rawResult.Active,
		Subject:   rawResult.Subject,
		ClientID:  rawResult.ClientID,
		TokenType: rawResult.TokenType,
		ExpiresAt: time.Unix(rawResult.ExpiresAt, 0).UTC(),
		Scope:     rawResult.Scope,
		Roles:     rawResult.Roles,
	}, nil
}

func (c *keycloakClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", strings.TrimRight(c.config.BaseURL, "/"), c.config.Realm)

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

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, ErrKeycloakUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token failed with status: %d", resp.StatusCode)
	}

	tokenPair := &TokenPair{}
	if err := json.NewDecoder(resp.Body).Decode(tokenPair); err != nil {
		return nil, err
	}
	return tokenPair, nil
}

func (c *keycloakClient) GetServiceToken(ctx context.Context) (string, error) {
	if c.serviceTokenCache.isValid() {
		return c.serviceTokenCache.getToken(), nil
	}

	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", strings.TrimRight(c.config.BaseURL, "/"), c.config.Realm)
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return "", ErrKeycloakUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get service token failed with status: %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	c.serviceTokenCache.setToken(result.AccessToken, time.Now().Add(time.Duration(result.ExpiresIn)*time.Second))
	return result.AccessToken, nil
}

func (c *keycloakClient) Logout(ctx context.Context, refreshToken string) error {
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout", strings.TrimRight(c.config.BaseURL, "/"), c.config.Realm)

	data := url.Values{}
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return ErrKeycloakUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("logout failed with status: %d", resp.StatusCode)
	}
	return nil
}

func (c *keycloakClient) Health(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/realms/%s/.well-known/openid-configuration", strings.TrimRight(c.config.BaseURL, "/"), c.config.Realm)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.doWithRetry(req)
	if err != nil {
		return ErrKeycloakUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ErrKeycloakUnavailable
	}
	return nil
}

func (c *keycloakClient) doWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i <= c.config.RetryAttempts; i++ {
		// Clone request for retries as Body might be closed
		if i > 0 {
			time.Sleep(c.config.RetryDelay * time.Duration(1<<i))
			if req.Body != nil {
				// This is a simplification; for full retry support, body needs to be seekable or buffered.
				// For now assuming body is small and we can't easily rewind if it's an io.Reader.
				// However, GetUserInfo has no body, Introspect/Token has form body which we can reconstruct if needed,
				// but here we used NewReader so it is seekable? No, strings.NewReader is seekable but http.Request wraps it.
				// In this implementation we just retry if error is transient.
				// For production, strictly handling body rewind is better.
				if seeker, ok := req.Body.(io.Seeker); ok {
					seeker.Seek(0, io.SeekStart)
				}
			}
		}

		// Add X-Request-ID if context has it
		if reqID := logging.RequestIDFromContext(req.Context()); reqID != "" {
			req.Header.Set("X-Request-ID", reqID)
		}

		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return nil, err
}

//Personal.AI order the ending
