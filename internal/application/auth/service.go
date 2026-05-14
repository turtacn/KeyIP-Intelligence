// Auth application service: sign-in with bcrypt password verification + JWT token issuance.
// No Keycloak required — suitable for local development and docker-machine setups.

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/user"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// Service handles local authentication (sign-in, JWT issuance, token validation).
type Service struct {
	userRepo   user.UserRepository
	jwtSecret  []byte
	jwtTTL     time.Duration
	logger     logging.Logger
}

// ServiceConfig contains values for constructing an auth Service.
type ServiceConfig struct {
	JWTSecret string
	JWTIssuer string
	JWTTTL    time.Duration
}

// SignInRequest is the payload for POST /api/v1/auth/signin.
type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SignInResponse is returned on successful authentication.
type SignInResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"` // reserve for future use
}

// TokenClaims are the custom JWT claims embedded in every access token.
type TokenClaims struct {
	jwt.RegisteredClaims
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Username string   `json:"preferred_username"`
	Name     string   `json:"name"`
	Roles    []string `json:"roles"`
}

// NewService creates a new auth Service.
func NewService(cfg ServiceConfig, userRepo user.UserRepository, logger logging.Logger) *Service {
	if cfg.JWTTTL <= 0 {
		cfg.JWTTTL = 24 * time.Hour
	}
	return &Service{
		userRepo:  userRepo,
		jwtSecret: []byte(cfg.JWTSecret),
		jwtTTL:    cfg.JWTTTL,
		logger:    logger,
	}
}

// SignIn authenticates a user by email+password and returns a JWT access token.
func (s *Service) SignIn(ctx context.Context, req SignInRequest) (*SignInResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, errors.New(errors.ErrCodeBadRequest, "email and password are required")
	}

	u, err := s.userRepo.GetByEmailForAuth(ctx, req.Email)
	if err != nil {
		s.logger.Warn("sign-in: user not found", logging.String("email", req.Email))
		return nil, errors.New(errors.ErrCodeUnauthorized, "invalid credentials")
	}

	if u.Status != "active" {
		return nil, errors.New(errors.ErrCodeUnauthorized, "account is not active")
	}

	if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
		return nil, errors.New(errors.ErrCodeUnauthorized, "account is temporarily locked")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		// Increment failed login count (best-effort)
		if incErr := s.userRepo.IncrementFailedLogin(ctx, u.ID); incErr != nil {
			s.logger.Error("failed to increment failed login", logging.Err(incErr))
		}
		s.logger.Warn("sign-in: password mismatch", logging.String("email", req.Email))
		return nil, errors.New(errors.ErrCodeUnauthorized, "invalid credentials")
	}

	// Success: update login info
	clientIP := clientIPFromContext(ctx)
	if updateErr := s.userRepo.UpdateLoginInfo(ctx, u.ID, clientIP); updateErr != nil {
		s.logger.Warn("failed to update last login", logging.Err(updateErr))
	}

	now := time.Now()
	expiresAt := now.Add(s.jwtTTL)
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "keyip-apiserver",
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.New().String(),
		},
		UserID:   u.ID.String(),
		Email:    u.Email,
		Username: u.Username,
		Name:     u.DisplayName,
		Roles:    []string{"user"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, signErr := token.SignedString(s.jwtSecret)
	if signErr != nil {
		return nil, errors.Wrap(signErr, errors.ErrCodeInternal, "failed to sign JWT")
	}

	return &SignInResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.jwtTTL.Seconds()),
	}, nil
}

// ValidateToken parses and validates a JWT access token string, returning the claims.
func (s *Service) ValidateToken(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return s.jwtSecret, nil
		},
		jwt.WithIssuer("keyip-apiserver"),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, errors.New(errors.ErrCodeUnauthorized, "invalid or expired token")
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, errors.New(errors.ErrCodeUnauthorized, "invalid token claims")
	}
	return claims, nil
}

// GetUserProfile returns the user profile for the current authenticated token.
func (s *Service) GetUserProfile(ctx context.Context, tokenStr string) (*user.User, error) {
	claims, err := s.ValidateToken(tokenStr)
	if err != nil {
		return nil, err
	}
	uid, parseErr := uuid.Parse(claims.UserID)
	if parseErr != nil {
		return nil, errors.New(errors.ErrCodeUnauthorized, "invalid user id in token")
	}
	return s.userRepo.GetByID(ctx, uid)
}

// GenerateRandomSecret creates a random 32-byte hex-encoded secret suitable for JWT signing.
func GenerateRandomSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(buf)
}

// clientIPFromContext extracts the client IP for audit logging.
func clientIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value("client_ip").(string); ok {
		return v
	}
	return ""
}

//Personal.AI order the ending
