package keycloak

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
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
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Mock Logger
type mockLogger struct {
	logging.Logger
}

func (m *mockLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Debug(msg string, fields ...logging.Field) {}

func newMockLogger() logging.Logger {
	return &mockLogger{}
}

func setupTestKeycloak(t *testing.T) (*httptest.Server, AuthProvider, *rsa.PrivateKey) {
	// Generate RSA Key Pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	// JWKS Handler
	mux.HandleFunc("/realms/test-realm/protocol/openid-connect/certs", func(w http.ResponseWriter, r *http.Request) {
		n := base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes())
		// e is usually 65537
		eBytes := []byte{1, 0, 1}
		e := base64.RawURLEncoding.EncodeToString(eBytes)

		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": "test-key-id",
					"n":   n,
					"e":   e,
					"alg": "RS256",
					"use": "sig",
				},
			},
		}
		json.NewEncoder(w).Encode(jwks)
	})

	// UserInfo Handler
	mux.HandleFunc("/realms/test-realm/protocol/openid-connect/userinfo", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "invalid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if token == "error" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		userInfo := UserInfo{
			ID:    "user-123",
			Email: "test@example.com",
			Name:  "Test User",
		}
		json.NewEncoder(w).Encode(userInfo)
	})

	// Introspect Handler
	mux.HandleFunc("/realms/test-realm/protocol/openid-connect/token/introspect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		token := r.FormValue("token")
		active := token == "active-token"
		result := IntrospectionResult{
			Active: active,
			Subject: "user-123",
		}
		json.NewEncoder(w).Encode(result)
	})

	// Token Handler
	mux.HandleFunc("/realms/test-realm/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		grantType := r.FormValue("grant_type")
		if grantType == "client_credentials" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "service-token",
				"expires_in":   300,
			})
			return
		}
		if grantType == "refresh_token" {
			refreshToken := r.FormValue("refresh_token")
			if refreshToken == "valid-refresh" {
				json.NewEncoder(w).Encode(TokenPair{
					AccessToken:  "new-access",
					RefreshToken: "new-refresh",
					ExpiresIn:    300,
				})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
	})

	// Logout Handler
	mux.HandleFunc("/realms/test-realm/protocol/openid-connect/logout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Health Handler
	mux.HandleFunc("/realms/test-realm/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := KeycloakConfig{
		BaseURL:                  server.URL, // httptest server URL doesn't have trailing slash usually
		Realm:                    "test-realm",
		ClientID:                 "test-client",
		ClientSecret:             "secret",
		PublicKeyRefreshInterval: 1 * time.Second,
	}

	client, err := NewKeycloakClient(cfg, newMockLogger())
	require.NoError(t, err)

	return server, client, privateKey
}

func signTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key-id"
	signed, err := token.SignedString(privateKey)
	require.NoError(t, err)
	return signed
}

func TestNewKeycloakClient_ValidConfig(t *testing.T) {
	server, client, _ := setupTestKeycloak(t)
	defer server.Close()
	assert.NotNil(t, client)
}

func TestNewKeycloakClient_InvalidConfig(t *testing.T) {
	_, err := NewKeycloakClient(KeycloakConfig{}, newMockLogger())
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrCodeInternal))
}

func TestNewKeycloakClient_TrailingSlash(t *testing.T) {
	// Setup minimalist server for JWKS
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/test-realm/protocol/openid-connect/certs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"keys":[]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := KeycloakConfig{
		BaseURL:      server.URL + "/",
		Realm:        "test-realm",
		ClientID:     "id",
		ClientSecret: "sec",
	}
	client, err := NewKeycloakClient(cfg, newMockLogger())
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestVerifyToken_ValidToken(t *testing.T) {
	server, client, key := setupTestKeycloak(t)
	defer server.Close()

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": server.URL + "/realms/test-realm",
		"aud": []string{"test-client"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"realm_access": map[string]interface{}{
			"roles": []string{"admin"},
		},
	}
	token := signTestToken(t, key, claims)

	tc, err := client.VerifyToken(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", tc.Subject)
	assert.Contains(t, tc.RealmRoles, "admin")
}

func TestVerifyToken_ExpiredToken(t *testing.T) {
	server, client, key := setupTestKeycloak(t)
	defer server.Close()

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": server.URL + "/realms/test-realm",
		"aud": []string{"test-client"},
		"exp": time.Now().Add(-time.Hour).Unix(),
	}
	token := signTestToken(t, key, claims)

	_, err := client.VerifyToken(context.Background(), token)
	assert.Error(t, err)
	assert.Equal(t, ErrTokenExpired, err)
}

func TestGetUserInfo_Success(t *testing.T) {
	server, client, _ := setupTestKeycloak(t)
	defer server.Close()

	info, err := client.GetUserInfo(context.Background(), "valid-token")
	require.NoError(t, err)
	assert.Equal(t, "user-123", info.ID)
}

func TestGetUserInfo_Retry(t *testing.T) {
	// Create a server that fails 2 times then succeeds
	failures := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/test-realm/protocol/openid-connect/certs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"keys":[]}`))
	})
	mux.HandleFunc("/realms/test-realm/protocol/openid-connect/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if failures < 2 {
			failures++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sub":"user-123"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := KeycloakConfig{
		BaseURL:       server.URL,
		Realm:         "test-realm",
		ClientID:      "id",
		ClientSecret:  "sec",
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond,
	}
	client, err := NewKeycloakClient(cfg, newMockLogger())
	require.NoError(t, err)

	info, err := client.GetUserInfo(context.Background(), "token")
	require.NoError(t, err)
	assert.Equal(t, "user-123", info.ID)
	assert.Equal(t, 2, failures)
}

func TestGetUserInfo_Unauthorized(t *testing.T) {
	server, client, _ := setupTestKeycloak(t)
	defer server.Close()

	_, err := client.GetUserInfo(context.Background(), "invalid")
	assert.Error(t, err)
	assert.Equal(t, ErrTokenExpired, err)
}

func TestIntrospectToken_Active(t *testing.T) {
	server, client, _ := setupTestKeycloak(t)
	defer server.Close()

	res, err := client.IntrospectToken(context.Background(), "active-token")
	require.NoError(t, err)
	assert.True(t, res.Active)
}

func TestRefreshToken_Success(t *testing.T) {
	server, client, _ := setupTestKeycloak(t)
	defer server.Close()

	pair, err := client.RefreshToken(context.Background(), "valid-refresh")
	require.NoError(t, err)
	assert.Equal(t, "new-access", pair.AccessToken)
}

func TestGetServiceToken_Success(t *testing.T) {
	server, client, _ := setupTestKeycloak(t)
	defer server.Close()

	token, err := client.GetServiceToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "service-token", token)

	// Cached
	token2, err := client.GetServiceToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, token, token2)
}

func TestLogout_Success(t *testing.T) {
	server, client, _ := setupTestKeycloak(t)
	defer server.Close()

	err := client.Logout(context.Background(), "refresh-token")
	require.NoError(t, err)
}

func TestHealth_Healthy(t *testing.T) {
	server, client, _ := setupTestKeycloak(t)
	defer server.Close()

	err := client.Health(context.Background())
	require.NoError(t, err)
}

func TestJWKSCache_ConcurrentRead(t *testing.T) {
	server, client, key := setupTestKeycloak(t)
	defer server.Close()

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": server.URL + "/realms/test-realm",
		"aud": []string{"test-client"},
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := signTestToken(t, key, claims)

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
