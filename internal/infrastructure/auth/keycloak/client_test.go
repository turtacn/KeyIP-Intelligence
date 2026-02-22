package keycloak

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	stdliberrors "errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type mockKeycloak struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	kid        string
	tokenSub   string
	tokenExp   time.Time
}

func setupTestKeycloak(t *testing.T) (*mockKeycloak, AuthProvider) {
	// Generate RSA keys
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey
	kid := "test-key-id"

	mk := &mockKeycloak{
		privateKey: privateKey,
		publicKey:  publicKey,
		kid:        kid,
		tokenSub:   "test-user",
		tokenExp:   time.Now().Add(1 * time.Hour),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/openid-connect/certs"):
			mk.handleJWKS(w, r)
		case strings.Contains(r.URL.Path, "/openid-connect/userinfo"):
			mk.handleUserInfo(w, r)
		case strings.Contains(r.URL.Path, "/openid-connect/token/introspect"):
			mk.handleIntrospect(w, r)
		case strings.Contains(r.URL.Path, "/openid-connect/token"):
			mk.handleToken(w, r)
		case strings.Contains(r.URL.Path, "/openid-connect/logout"):
			mk.handleLogout(w, r)
		case strings.Contains(r.URL.Path, "/.well-known/openid-configuration"):
			mk.handleOpenIDConfig(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	mk.server = httptest.NewServer(handler)

	cfg := KeycloakConfig{
		BaseURL:                  mk.server.URL,
		Realm:                    "test-realm",
		ClientID:                 "test-client",
		ClientSecret:             "test-secret",
		PublicKeyRefreshInterval: 100 * time.Millisecond,
		RequestTimeout:           1 * time.Second,
		RetryAttempts:            1,
		RetryDelay:               1 * time.Millisecond,
	}

	client, err := NewKeycloakClient(cfg, logging.NewNopLogger())
	require.NoError(t, err)

	return mk, client
}

func (mk *mockKeycloak) handleJWKS(w http.ResponseWriter, r *http.Request) {
	nBytes := mk.publicKey.N.Bytes()
	eBytes := big.NewInt(int64(mk.publicKey.E)).Bytes()

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kid": mk.kid,
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(nBytes),
				"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			},
		},
	}
	json.NewEncoder(w).Encode(jwks)
}

func (mk *mockKeycloak) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	// Simplified logic: assume valid token for now
	userInfo := map[string]interface{}{
		"sub":                mk.tokenSub,
		"email":              "test@example.com",
		"email_verified":     true,
		"name":               "Test User",
		"preferred_username": "testuser",
		"given_name":         "Test",
		"family_name":        "User",
		"roles":              []string{"role1", "role2"},
		"groups":             []string{"group1"},
		"tenant_id":          "tenant-1",
		"attributes": map[string]interface{}{
			"attr1": []string{"val1"},
		},
	}
	json.NewEncoder(w).Encode(userInfo)
}

func (mk *mockKeycloak) handleIntrospect(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	token := r.Form.Get("token")
	active := token == "valid-token"

	result := map[string]interface{}{
		"active":    active,
		"sub":       mk.tokenSub,
		"client_id": "test-client",
		"exp":       mk.tokenExp.Unix(),
		"scope":     "openid profile email",
	}
	json.NewEncoder(w).Encode(result)
}

func (mk *mockKeycloak) handleToken(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	grantType := r.Form.Get("grant_type")

	if grantType == "refresh_token" {
		refreshToken := r.Form.Get("refresh_token")
		if refreshToken == "invalid-refresh" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":       "new-access-token",
			"refresh_token":      "new-refresh-token",
			"expires_in":         3600,
			"refresh_expires_in": 7200,
			"token_type":         "Bearer",
		})
		return
	}

	if grantType == "client_credentials" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "service-token",
			"expires_in":   3600,
		})
		return
	}

	w.WriteHeader(http.StatusBadRequest)
}

func (mk *mockKeycloak) handleLogout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (mk *mockKeycloak) handleOpenIDConfig(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"issuer": mk.server.URL + "/realms/test-realm",
	})
}

func (mk *mockKeycloak) signToken(claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = mk.kid
	signedToken, _ := token.SignedString(mk.privateKey)
	return signedToken
}

func TestNewKeycloakClient_ValidConfig(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()
	assert.NotNil(t, client)
}

func TestNewKeycloakClient_MissingConfig(t *testing.T) {
	_, err := NewKeycloakClient(KeycloakConfig{}, logging.NewNopLogger())
	assert.Error(t, err)
	assert.True(t, stdliberrors.Is(err, ErrInvalidConfig))
}

func TestVerifyToken_ValidToken(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	claims := jwt.MapClaims{
		"sub":                "user-123",
		"iss":                mk.server.URL + "/realms/test-realm",
		"aud":                []string{"test-client"},
		"exp":                time.Now().Add(time.Hour).Unix(),
		"iat":                time.Now().Unix(),
		"email":              "user@example.com",
		"preferred_username": "user123",
		"realm_access": map[string]interface{}{
			"roles": []string{"admin"},
		},
		"resource_access": map[string]interface{}{
			"test-client": map[string]interface{}{
				"roles": []string{"manager"},
			},
		},
		"tenant_id": "tenant-abc",
	}
	token := mk.signToken(claims)

	tokenClaims, err := client.VerifyToken(context.Background(), token)
	assert.NoError(t, err)
	assert.NotNil(t, tokenClaims)
	assert.Equal(t, "user-123", tokenClaims.Subject)
	assert.Equal(t, "user@example.com", tokenClaims.Email)
	assert.Contains(t, tokenClaims.RealmRoles, "admin")
	assert.Contains(t, tokenClaims.ClientRoles["test-client"], "manager")
	assert.Equal(t, "tenant-abc", tokenClaims.TenantID)
}

func TestVerifyToken_ExpiredToken(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": mk.server.URL + "/realms/test-realm",
		"aud": []string{"test-client"},
		"exp": time.Now().Add(-time.Hour).Unix(),
	}
	token := mk.signToken(claims)

	_, err := client.VerifyToken(context.Background(), token)
	assert.Error(t, err)
	assert.True(t, stdliberrors.Is(err, ErrTokenExpired))
}

func TestVerifyToken_InvalidSignature(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": mk.server.URL + "/realms/test-realm",
		"aud": []string{"test-client"},
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	// Sign with a different key
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = mk.kid // Use same kid but wrong key signature
	signedToken, _ := token.SignedString(otherKey)

	_, err := client.VerifyToken(context.Background(), tokenString(signedToken)) // tokenString cast just to be safe
	// Actually signedToken is string.
	_, err = client.VerifyToken(context.Background(), signedToken)
	assert.Error(t, err)
	// It might return InvalidSignature or VerificationFailed depending on jwt lib behavior with wrong key but matching kid
	// Since we verify with the public key corresponding to kid, verification should fail.
	assert.True(t, stdliberrors.Is(err, ErrTokenInvalidSignature))
}

func tokenString(s string) string { return s }

func TestGetUserInfo_Success(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	userInfo, err := client.GetUserInfo(context.Background(), "valid-token")
	assert.NoError(t, err)
	assert.NotNil(t, userInfo)
	assert.Equal(t, "test-user", userInfo.ID)
}

func TestIntrospectToken_Active(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	result, err := client.IntrospectToken(context.Background(), "valid-token")
	assert.NoError(t, err)
	assert.True(t, result.Active)
}

func TestIntrospectToken_Inactive(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	result, err := client.IntrospectToken(context.Background(), "invalid-token")
	assert.NoError(t, err)
	assert.False(t, result.Active)
}

func TestRefreshToken_Success(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	pair, err := client.RefreshToken(context.Background(), "valid-refresh")
	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", pair.AccessToken)
}

func TestGetServiceToken_Success(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	token, err := client.GetServiceToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "service-token", token)

	// Second call should hit cache
	// We can't easily verify cache hit without exposing internal state or mocking http client more deeply
	// But we can check if it returns same token
	token2, err := client.GetServiceToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, token, token2)
}

func TestHealth_Healthy(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	err := client.Health(context.Background())
	assert.NoError(t, err)
}

func TestHealth_Unhealthy(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	mk.server.Close() // Close server to simulate failure
	// We need to keep defer in setupTestKeycloak but here we close it early.
	// setupTestKeycloak defers close? No, test function defers close.
	// So we can close it here.

	// However, if we close it, client requests will fail.
	err := client.Health(context.Background())
	assert.Error(t, err)
	assert.True(t, stdliberrors.Is(err, ErrKeycloakUnavailable))
}

func TestJWKSCache_ConcurrentRead(t *testing.T) {
	mk, client := setupTestKeycloak(t)
	defer mk.server.Close()

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": mk.server.URL + "/realms/test-realm",
		"aud": []string{"test-client"},
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := mk.signToken(claims)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.VerifyToken(context.Background(), token)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}

//Personal.AI order the ending
