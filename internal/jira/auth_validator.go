package jira

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/atlet99/gitlab-jira-hook/internal/errors"
)

// AuthValidator provides unified authentication validation for Jira webhooks
type AuthValidator struct {
	logger          *slog.Logger
	hmacSecret      string
	jwtValidator    *JWTValidator
	requireAuth     bool
	developmentMode bool
}

// AuthValidationResult contains the result of authentication validation
type AuthValidationResult struct {
	Valid           bool
	AuthType        AuthType
	ErrorCode       errors.ErrorCode
	ErrorMessage    string
	JWTClaims       *JWTClaims
	UserID          string
	IssuerClientKey string
}

// AuthType represents the type of authentication used
type AuthType string

const (
	// AuthTypeNone indicates no authentication is provided
	AuthTypeNone AuthType = "none" // No authentication
	// AuthTypeHMAC indicates HMAC-SHA256 signature authentication
	AuthTypeHMAC AuthType = "hmac" // HMAC-SHA256 signature
	// AuthTypeJWT indicates JWT token authentication for Connect apps
	AuthTypeJWT AuthType = "jwt" // JWT token (Connect apps)
	// AuthTypeDevelopment indicates development mode bypass
	AuthTypeDevelopment AuthType = "development" // Development mode bypass
)

// NewAuthValidator creates a new authentication validator
func NewAuthValidator(logger *slog.Logger, hmacSecret string, developmentMode bool) *AuthValidator {
	return &AuthValidator{
		logger:          logger,
		hmacSecret:      hmacSecret,
		jwtValidator:    NewJWTValidator(logger, "", []string{}), // Will be configured as needed
		requireAuth:     hmacSecret != "",
		developmentMode: developmentMode,
	}
}

// WithJWTValidator configures JWT validation for Connect apps
func (v *AuthValidator) WithJWTValidator(expectedAudience string, allowedIssuers []string) *AuthValidator {
	v.jwtValidator = NewJWTValidator(v.logger, expectedAudience, allowedIssuers)
	return v
}

// ValidateRequest validates authentication for incoming Jira webhook requests
func (v *AuthValidator) ValidateRequest(ctx context.Context, r *http.Request, body []byte) *AuthValidationResult {
	// Development mode bypass
	if v.developmentMode {
		v.logger.Debug("Development mode: skipping authentication validation")
		return &AuthValidationResult{
			Valid:    true,
			AuthType: AuthTypeDevelopment,
		}
	}

	// Check for JWT authentication first
	authHeader := r.Header.Get("Authorization")
	if v.jwtValidator.IsJWTRequest(r) {
		return v.validateJWTAuth(ctx, authHeader, r)
	}

	// Check for HMAC signature authentication
	if v.hasHMACSignature(r) {
		return v.validateHMACAuth(r, body)
	}

	// No authentication provided
	if !v.requireAuth {
		v.logger.Debug("No authentication required, allowing request")
		return &AuthValidationResult{
			Valid:    true,
			AuthType: AuthTypeNone,
		}
	}

	// Authentication required but not provided
	return &AuthValidationResult{
		Valid:        false,
		AuthType:     AuthTypeNone,
		ErrorCode:    errors.ErrCodeInvalidSignature,
		ErrorMessage: "Authentication required but not provided",
	}
}

// validateJWTAuth validates JWT token authentication
func (v *AuthValidator) validateJWTAuth(ctx context.Context, authHeader string, r *http.Request) *AuthValidationResult {
	v.logger.Debug("Validating JWT authentication")

	jwtResult := v.jwtValidator.ValidateJWT(ctx, authHeader, r)

	result := &AuthValidationResult{
		Valid:        jwtResult.Valid,
		AuthType:     AuthTypeJWT,
		ErrorCode:    jwtResult.ErrorCode,
		ErrorMessage: jwtResult.ErrorMessage,
		JWTClaims:    jwtResult.Claims,
	}

	if jwtResult.Valid && jwtResult.Claims != nil {
		result.IssuerClientKey = jwtResult.Claims.Issuer
		result.UserID = jwtResult.Claims.Subject

		v.logger.Info("JWT authentication successful",
			"tokenType", jwtResult.TokenType,
			"issuer", jwtResult.Claims.Issuer,
			"subject", jwtResult.Claims.Subject)
	} else {
		v.logger.Warn("JWT authentication failed",
			"error", FormatValidationError(jwtResult))
	}

	return result
}

// validateHMACAuth validates HMAC signature authentication
func (v *AuthValidator) validateHMACAuth(r *http.Request, body []byte) *AuthValidationResult {
	v.logger.Debug("Validating HMAC signature authentication")

	if v.hmacSecret == "" {
		return &AuthValidationResult{
			Valid:        false,
			AuthType:     AuthTypeHMAC,
			ErrorCode:    errors.ErrCodeInvalidSignature,
			ErrorMessage: "HMAC secret not configured",
		}
	}

	// Get signature from headers
	signature := v.getSignatureFromHeaders(r)
	if signature == "" {
		return &AuthValidationResult{
			Valid:        false,
			AuthType:     AuthTypeHMAC,
			ErrorCode:    errors.ErrCodeInvalidSignature,
			ErrorMessage: "No HMAC signature found in request headers",
		}
	}

	// Validate signature
	if v.validateHMACSignature(signature, body) {
		v.logger.Info("HMAC authentication successful")
		return &AuthValidationResult{
			Valid:    true,
			AuthType: AuthTypeHMAC,
		}
	}

	v.logger.Warn("HMAC signature validation failed",
		"expectedLength", len(v.computeHMACSignature(body)),
		"receivedLength", len(signature))

	return &AuthValidationResult{
		Valid:        false,
		AuthType:     AuthTypeHMAC,
		ErrorCode:    errors.ErrCodeInvalidSignature,
		ErrorMessage: "HMAC signature validation failed",
	}
}

// hasHMACSignature checks if the request has HMAC signature headers
func (v *AuthValidator) hasHMACSignature(r *http.Request) bool {
	signatures := []string{
		"X-Atlassian-Webhook-Signature",
		"X-Hub-Signature-256",
		"X-Signature",
	}

	for _, header := range signatures {
		if r.Header.Get(header) != "" {
			return true
		}
	}
	return false
}

// getSignatureFromHeaders extracts HMAC signature from request headers
func (v *AuthValidator) getSignatureFromHeaders(r *http.Request) string {
	// Try different signature header formats
	signatures := map[string]string{
		"X-Atlassian-Webhook-Signature": "",
		"X-Hub-Signature-256":           "sha256=",
		"X-Signature":                   "",
	}

	for header, prefix := range signatures {
		if signature := r.Header.Get(header); signature != "" {
			// Remove prefix if present
			if prefix != "" && strings.HasPrefix(signature, prefix) {
				signature = strings.TrimPrefix(signature, prefix)
			}
			return signature
		}
	}

	return ""
}

// validateHMACSignature validates HMAC-SHA256 signature
func (v *AuthValidator) validateHMACSignature(receivedSignature string, body []byte) bool {
	expectedSignature := v.computeHMACSignature(body)

	// Use constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(expectedSignature), []byte(receivedSignature))
}

// computeHMACSignature computes HMAC-SHA256 signature for the given body
func (v *AuthValidator) computeHMACSignature(body []byte) string {
	mac := hmac.New(sha256.New, []byte(v.hmacSecret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// RequiresAuthentication returns true if authentication is required
func (v *AuthValidator) RequiresAuthentication() bool {
	return v.requireAuth && !v.developmentMode
}

// GetAuthenticationType determines the authentication type from request
func (v *AuthValidator) GetAuthenticationType(r *http.Request) AuthType {
	if v.developmentMode {
		return AuthTypeDevelopment
	}

	if v.jwtValidator.IsJWTRequest(r) {
		return AuthTypeJWT
	}

	if v.hasHMACSignature(r) {
		return AuthTypeHMAC
	}

	return AuthTypeNone
}

// IsConnectApp returns true if the request is from a Jira Connect app
func (v *AuthValidator) IsConnectApp(result *AuthValidationResult) bool {
	return result != nil && result.AuthType == AuthTypeJWT && result.Valid
}

// GetUserContext extracts user context from authentication result
func (v *AuthValidator) GetUserContext(result *AuthValidationResult) map[string]interface{} {
	userContext := make(map[string]interface{})

	if result == nil {
		return userContext
	}

	userContext["auth_type"] = string(result.AuthType)
	userContext["auth_valid"] = result.Valid

	if result.UserID != "" {
		userContext["user_id"] = result.UserID
	}

	if result.IssuerClientKey != "" {
		userContext["client_key"] = result.IssuerClientKey
	}

	if result.JWTClaims != nil {
		if result.JWTClaims.Context != nil {
			userContext["connect_context"] = result.JWTClaims.Context
		}
		userContext["issued_at"] = result.JWTClaims.IssuedAt
		userContext["expires_at"] = result.JWTClaims.ExpiresAt
	}

	return userContext
}

// CreateAuthError creates a structured authentication error
func (v *AuthValidator) CreateAuthError(result *AuthValidationResult, operation string) *errors.ServiceError {
	if result == nil || result.Valid {
		return nil
	}

	builder := errors.NewError(result.ErrorCode).
		WithCategory(errors.CategoryClientError).
		WithSeverity(errors.SeverityMedium).
		WithMessage(fmt.Sprintf("Authentication failed for %s", operation)).
		WithDetails(result.ErrorMessage).
		WithContext("auth_type", string(result.AuthType))

	switch result.ErrorCode {
	case errors.ErrCodeInvalidSignature:
		builder.WithUserMessage("Webhook signature validation failed. Please check your secret configuration.")
	case errors.ErrCodeJiraUnauthorized:
		builder.WithUserMessage("Jira authentication failed. Please check your app configuration.")
	case errors.ErrCodeValidationFailed:
		builder.WithUserMessage("Request validation failed. Please check your request format.")
	default:
		builder.WithUserMessage("Authentication failed. Please check your credentials.")
	}

	return builder.Build()
}

// LogAuthenticationAttempt logs authentication attempts for monitoring
func (v *AuthValidator) LogAuthenticationAttempt(r *http.Request, result *AuthValidationResult) {
	level := slog.LevelInfo
	if !result.Valid {
		level = slog.LevelWarn
	}

	attrs := []slog.Attr{
		slog.String("auth_type", string(result.AuthType)),
		slog.Bool("auth_valid", result.Valid),
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("user_agent", r.Header.Get("User-Agent")),
	}

	if result.UserID != "" {
		attrs = append(attrs, slog.String("user_id", result.UserID))
	}

	if result.IssuerClientKey != "" {
		attrs = append(attrs, slog.String("client_key", result.IssuerClientKey))
	}

	if !result.Valid {
		attrs = append(attrs, slog.String("error", result.ErrorMessage))
	}

	v.logger.LogAttrs(context.Background(), level, "Authentication attempt", attrs...)
}

// ValidateConnectAppPermissions validates permissions for Connect app operations
func (v *AuthValidator) ValidateConnectAppPermissions(result *AuthValidationResult, requiredScopes []string) error {
	if !v.IsConnectApp(result) {
		return fmt.Errorf("operation requires Connect app authentication")
	}

	// If no scopes are required, permission is granted
	if len(requiredScopes) == 0 {
		return nil
	}

	// Check if required scopes are available in JWT claims
	if result.JWTClaims != nil && result.JWTClaims.ScopesString != "" {
		// Split scopes string into individual scopes
		availableScopes := strings.Split(result.JWTClaims.ScopesString, " ")
		sort.Strings(availableScopes)
		for _, required := range requiredScopes {
			found := false
			for _, available := range availableScopes {
				if available == required {
					found = true
					break
				}
			}
			if !found {
				v.logger.Warn("Connect app missing required scope",
					"client_key", result.IssuerClientKey,
					"missing_scope", required,
					"available_scopes", availableScopes)

				return &errors.ServiceError{
					Code:     errors.ErrCodePermissionDenied,
					Message:  fmt.Sprintf("Missing required scope: %s", required),
					Details:  fmt.Sprintf("Connect app %s lacks required scope: %s", result.IssuerClientKey, required),
					Severity: errors.SeverityHigh,
					Category: errors.CategoryClientError,
				}
			}
		}
	} else {
		v.logger.Warn("Connect app has no scopes defined",
			"client_key", result.IssuerClientKey,
			"required_scopes", requiredScopes)

		return &errors.ServiceError{
			Code:     errors.ErrCodePermissionDenied,
			Message:  "Connect app has no defined scopes",
			Details:  "Application permissions not configured during installation",
			Severity: errors.SeverityHigh,
			Category: errors.CategoryClientError,
		}
	}

	v.logger.Debug("Connect app permissions validated",
		"client_key", result.IssuerClientKey,
		"required_scopes", requiredScopes)

	return nil
}

// Example usage patterns:
//
// 1. Basic validation:
//   validator := NewAuthValidator(logger, hmacSecret, false)
//   result := validator.ValidateRequest(ctx, request, body)
//   if !result.Valid {
//       return validator.CreateAuthError(result, "webhook processing")
//   }
//
// 2. Connect app validation:
//   validator := NewAuthValidator(logger, hmacSecret, false).
//       WithJWTValidator("https://your-app.com", []string{"known-client-keys"})
//   result := validator.ValidateRequest(ctx, request, body)
//   if validator.IsConnectApp(result) {
//       userID := result.UserID
//       clientKey := result.IssuerClientKey
//   }
//
// 3. Development mode:
//   validator := NewAuthValidator(logger, "", true) // Skip auth in dev
//   result := validator.ValidateRequest(ctx, request, body)
//   // Always valid in development mode
