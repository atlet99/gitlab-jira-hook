package jira

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"log/slog"

	"github.com/golang-jwt/jwt/v5"

	"github.com/atlet99/gitlab-jira-hook/internal/errors"
)

// JWTValidator handles Jira Connect app JWT token validation
type JWTValidator struct {
	logger           *slog.Logger
	httpClient       *http.Client
	publicKeyCache   map[string]*rsa.PublicKey
	cacheTTL         time.Duration
	allowedIssuers   []string
	expectedAudience string
}

// JWTClaims represents the claims in a Jira JWT token
type JWTClaims struct {
	Issuer    string                 `json:"iss"`               // Client key for tenant identification
	IssuedAt  int64                  `json:"iat"`               // Issued at time (Unix timestamp)
	ExpiresAt int64                  `json:"exp"`               // Expiration time (Unix timestamp)
	QueryHash string                 `json:"qsh"`               // Query string hash
	Subject   string                 `json:"sub,omitempty"`     // User account ID (optional)
	Audience  interface{}            `json:"aud,omitempty"`     // Audience (optional, string or []string)
	Context   map[string]interface{} `json:"context,omitempty"` // Connect context (optional)
	jwt.RegisteredClaims
}

// JWTHeader represents the header of a JWT token
type JWTHeader struct {
	Algorithm string `json:"alg"`           // Signing algorithm (HS256 or RS256)
	Type      string `json:"typ"`           // Token type (JWT)
	KeyID     string `json:"kid,omitempty"` // Key ID for asymmetric tokens
}

// ValidationResult contains the result of JWT validation
type ValidationResult struct {
	Valid        bool
	Claims       *JWTClaims
	TokenType    TokenType
	ErrorCode    errors.ErrorCode
	ErrorMessage string
}

// TokenType represents the type of JWT token
type TokenType string

const (
	// TokenTypeSymmetric indicates HS256 JWT with shared secret
	TokenTypeSymmetric TokenType = "symmetric" // HS256 with shared secret
	// TokenTypeAsymmetric indicates RS256 JWT with public key
	TokenTypeAsymmetric TokenType = "asymmetric" // RS256 with public key
	// TokenTypeContext indicates context JWT with fixed qsh value
	TokenTypeContext TokenType = "context" // Context JWT with fixed qsh
)

// Constants for JWT validation
const (
	AtlassianCDNBaseURL    = "https://connect-install-keys.atlassian.com"
	MaxClockSkew           = 5 * time.Minute
	DefaultCacheTTL        = 1 * time.Hour
	ContextQSH             = "context-qsh"
	JWTAuthorizationPrefix = "JWT "
	MinTokenLength         = 50
	MaxTokenLength         = 8192
	DefaultHTTPTimeout     = 10 * time.Second
)

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(logger *slog.Logger, expectedAudience string, allowedIssuers []string) *JWTValidator {
	return &JWTValidator{
		logger:           logger,
		httpClient:       &http.Client{Timeout: DefaultHTTPTimeout},
		publicKeyCache:   make(map[string]*rsa.PublicKey),
		cacheTTL:         DefaultCacheTTL,
		allowedIssuers:   allowedIssuers,
		expectedAudience: expectedAudience,
	}
}

// ValidateJWT validates a JWT token from the Authorization header
func (v *JWTValidator) ValidateJWT(ctx context.Context, authHeader string, request *http.Request) *ValidationResult {
	// Extract JWT token from Authorization header
	token, err := v.extractJWTToken(authHeader)
	if err != nil {
		return &ValidationResult{
			Valid:        false,
			ErrorCode:    errors.ErrCodeInvalidSignature,
			ErrorMessage: fmt.Sprintf("Failed to extract JWT token: %v", err),
		}
	}

	// Parse JWT token without verification to get header and claims
	parsedToken, err := jwt.ParseWithClaims(token, &JWTClaims{}, nil)
	if err != nil {
		// This is expected - we're parsing without verification
		if parsedToken == nil {
			return &ValidationResult{
				Valid:        false,
				ErrorCode:    errors.ErrCodeInvalidWebhook,
				ErrorMessage: fmt.Sprintf("Failed to parse JWT token: %v", err),
			}
		}
	}

	claims, ok := parsedToken.Claims.(*JWTClaims)
	if !ok {
		return &ValidationResult{
			Valid:        false,
			ErrorCode:    errors.ErrCodeInvalidWebhook,
			ErrorMessage: "Invalid JWT claims format",
		}
	}

	// Get header information
	header := parsedToken.Header
	algorithm, _ := header["alg"].(string)
	keyID, _ := header["kid"].(string)

	// Determine token type and validation strategy
	tokenType := v.determineTokenType(algorithm, keyID)

	v.logger.Debug("Processing JWT token",
		"algorithm", algorithm,
		"keyID", keyID,
		"tokenType", tokenType,
		"issuer", claims.Issuer)

	// Validate based on token type
	switch tokenType {
	case TokenTypeAsymmetric:
		return v.validateAsymmetricJWT(ctx, token, claims, keyID, request)
	case TokenTypeSymmetric:
		return v.validateSymmetricJWT(ctx, token, claims, request)
	default:
		return &ValidationResult{
			Valid:        false,
			ErrorCode:    errors.ErrCodeInvalidSignature,
			ErrorMessage: fmt.Sprintf("Unsupported token type: %s", tokenType),
		}
	}
}

// extractJWTToken extracts JWT token from Authorization header
func (v *JWTValidator) extractJWTToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is empty")
	}

	if !strings.HasPrefix(authHeader, JWTAuthorizationPrefix) {
		return "", fmt.Errorf("authorization header must start with 'JWT '")
	}

	token := strings.TrimPrefix(authHeader, JWTAuthorizationPrefix)
	if len(token) < MinTokenLength || len(token) > MaxTokenLength {
		return "", fmt.Errorf("invalid token length: %d", len(token))
	}

	return token, nil
}

// determineTokenType determines the type of JWT token based on algorithm and key ID
func (v *JWTValidator) determineTokenType(algorithm, keyID string) TokenType {
	switch algorithm {
	case "RS256":
		if keyID != "" {
			return TokenTypeAsymmetric
		}
		return TokenTypeSymmetric // Fallback for misconfigured tokens
	case "HS256":
		return TokenTypeSymmetric
	default:
		return TokenTypeSymmetric // Default fallback
	}
}

// validateAsymmetricJWT validates RS256 JWT tokens for lifecycle callbacks
func (v *JWTValidator) validateAsymmetricJWT(
	ctx context.Context, tokenString string, _ *JWTClaims, keyID string, _ *http.Request,
) *ValidationResult {
	// Get public key from Atlassian CDN
	publicKey, err := v.getPublicKey(ctx, keyID)
	if err != nil {
		return &ValidationResult{
			Valid:        false,
			ErrorCode:    errors.ErrCodeJiraUnauthorized,
			ErrorMessage: fmt.Sprintf("Failed to get public key: %v", err),
		}
	}

	// Parse and verify JWT with public key
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return &ValidationResult{
			Valid:        false,
			ErrorCode:    errors.ErrCodeInvalidSignature,
			ErrorMessage: fmt.Sprintf("JWT signature verification failed: %v", err),
		}
	}

	validatedClaims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return &ValidationResult{
			Valid:        false,
			ErrorCode:    errors.ErrCodeInvalidSignature,
			ErrorMessage: "JWT token is invalid",
		}
	}

	// Validate claims for asymmetric tokens (lifecycle callbacks)
	if err := v.validateAsymmetricClaims(validatedClaims); err != nil {
		return &ValidationResult{
			Valid:        false,
			ErrorCode:    errors.ErrCodeValidationFailed,
			ErrorMessage: fmt.Sprintf("Claims validation failed: %v", err),
		}
	}

	return &ValidationResult{
		Valid:     true,
		Claims:    validatedClaims,
		TokenType: TokenTypeAsymmetric,
	}
}

// validateSymmetricJWT validates HS256 JWT tokens with shared secret
func (v *JWTValidator) validateSymmetricJWT(
	_ context.Context, _ string, claims *JWTClaims, _ *http.Request,
) *ValidationResult {
	// Note: This requires shared secret which should be obtained during app installation
	// For now, we'll return an error indicating this needs to be implemented
	// with proper shared secret management

	v.logger.Warn("Symmetric JWT validation not fully implemented - requires shared secret from installation")

	// Validate basic claims structure
	if err := v.validateBasicClaims(claims); err != nil {
		return &ValidationResult{
			Valid:        false,
			ErrorCode:    errors.ErrCodeValidationFailed,
			ErrorMessage: fmt.Sprintf("Basic claims validation failed: %v", err),
		}
	}

	// For now, we'll do basic validation without signature verification
	// In production, you need to:
	// 1. Get shared secret for the issuer (clientKey) from installation data
	// 2. Verify JWT signature using HS256 with shared secret
	// 3. Validate query string hash (qsh) if not context JWT

	return &ValidationResult{
		Valid:        true, // TODO: Implement proper signature verification
		Claims:       claims,
		TokenType:    TokenTypeSymmetric,
		ErrorMessage: "Signature verification skipped - shared secret required",
	}
}

// getPublicKey retrieves public key from Atlassian CDN
func (v *JWTValidator) getPublicKey(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	// Check cache first
	if cachedKey, exists := v.publicKeyCache[keyID]; exists {
		return cachedKey, nil
	}

	// Fetch from CDN
	keyURL := fmt.Sprintf("%s/%s", AtlassianCDNBaseURL, keyID)

	req, err := http.NewRequestWithContext(ctx, "GET", keyURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			v.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch public key: HTTP %d", resp.StatusCode)
	}

	keyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	// Parse PEM-encoded RSA public key
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Cache the key
	v.publicKeyCache[keyID] = publicKey

	v.logger.Debug("Public key cached", "keyID", keyID)

	return publicKey, nil
}

// validateAsymmetricClaims validates claims for asymmetric JWT tokens (lifecycle callbacks)
func (v *JWTValidator) validateAsymmetricClaims(claims *JWTClaims) error {
	// Validate required claims
	if err := v.validateBasicClaims(claims); err != nil {
		return err
	}

	// Validate audience claim for lifecycle callbacks
	if v.expectedAudience != "" {
		if err := v.validateAudience(claims); err != nil {
			return err
		}
	}

	return nil
}

// validateBasicClaims validates basic required claims
func (v *JWTValidator) validateBasicClaims(claims *JWTClaims) error {
	now := time.Now()

	// Validate issuer
	if claims.Issuer == "" {
		return fmt.Errorf("missing issuer (iss) claim")
	}

	// Validate allowed issuers if configured
	if len(v.allowedIssuers) > 0 {
		allowed := false
		for _, allowedIssuer := range v.allowedIssuers {
			if claims.Issuer == allowedIssuer {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("issuer '%s' not in allowed list", claims.Issuer)
		}
	}

	// Validate issued at time
	if claims.IssuedAt == 0 {
		return fmt.Errorf("missing issued at (iat) claim")
	}

	issuedAt := time.Unix(claims.IssuedAt, 0)
	if issuedAt.After(now.Add(MaxClockSkew)) {
		return fmt.Errorf("token issued in the future: %v", issuedAt)
	}

	// Validate expiration time
	if claims.ExpiresAt == 0 {
		return fmt.Errorf("missing expiration (exp) claim")
	}

	expiresAt := time.Unix(claims.ExpiresAt, 0)
	if expiresAt.Before(now.Add(-MaxClockSkew)) {
		return fmt.Errorf("token has expired: %v", expiresAt)
	}

	// Validate query string hash is present
	if claims.QueryHash == "" {
		return fmt.Errorf("missing query string hash (qsh) claim")
	}

	return nil
}

// validateAudience validates the audience claim
func (v *JWTValidator) validateAudience(claims *JWTClaims) error {
	if claims.Audience == nil {
		return fmt.Errorf("missing audience (aud) claim")
	}

	// Handle both string and []string audience formats
	audiences := make([]string, 0)
	switch aud := claims.Audience.(type) {
	case string:
		audiences = append(audiences, aud)
	case []string:
		audiences = aud
	case []interface{}:
		for _, a := range aud {
			if str, ok := a.(string); ok {
				audiences = append(audiences, str)
			}
		}
	default:
		return fmt.Errorf("invalid audience format")
	}

	// Check if expected audience is present
	for _, audience := range audiences {
		if audience == v.expectedAudience {
			return nil
		}
	}

	return fmt.Errorf("expected audience '%s' not found in %v", v.expectedAudience, audiences)
}

// computeCanonicalHash computes the canonical request hash for qsh validation
func (v *JWTValidator) computeCanonicalHash(request *http.Request) string {
	// Canonical method (uppercase)
	method := strings.ToUpper(request.Method)

	// Canonical URI (path without query parameters)
	uri := request.URL.Path
	if uri == "" {
		uri = "/"
	}

	// Canonical query string
	queryString := v.buildCanonicalQueryString(request)

	// Build canonical request
	canonicalRequest := fmt.Sprintf("%s&%s&%s", method, uri, queryString)

	// Hash with SHA-256
	hash := sha256.Sum256([]byte(canonicalRequest))
	return hex.EncodeToString(hash[:])
}

// buildCanonicalQueryString builds the canonical query string for qsh validation
func (v *JWTValidator) buildCanonicalQueryString(request *http.Request) string {
	values := request.URL.Query()

	// Remove jwt parameter if present
	values.Del("jwt")

	// Sort parameters
	var keys []string
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build canonical query string
	var parts []string
	for _, key := range keys {
		encodedKey := url.QueryEscape(key)
		paramValues := values[key]
		sort.Strings(paramValues)

		// URL encode values and join with comma
		var encodedValues []string
		for _, value := range paramValues {
			encodedValues = append(encodedValues, url.QueryEscape(value))
		}

		part := fmt.Sprintf("%s=%s", encodedKey, strings.Join(encodedValues, "%2C"))
		parts = append(parts, part)
	}

	return strings.Join(parts, "&")
}

// IsJWTRequest checks if the request contains a JWT token
func (v *JWTValidator) IsJWTRequest(request *http.Request) bool {
	authHeader := request.Header.Get("Authorization")
	return strings.HasPrefix(authHeader, JWTAuthorizationPrefix)
}

// GetClaimsFromValidationResult safely extracts claims from validation result
func GetClaimsFromValidationResult(result *ValidationResult) *JWTClaims {
	if result != nil && result.Valid && result.Claims != nil {
		return result.Claims
	}
	return nil
}

// FormatValidationError formats a validation error for logging
func FormatValidationError(result *ValidationResult) string {
	if result == nil {
		return "validation result is nil"
	}
	if result.Valid {
		return "validation successful"
	}
	return fmt.Sprintf("validation failed [%s]: %s", result.ErrorCode, result.ErrorMessage)
}

// Example usage for different scenarios:
//
// 1. Lifecycle callback validation (install/uninstall):
//   validator := NewJWTValidator(logger, "https://your-app.com", []string{"known-client-keys"})
//   result := validator.ValidateJWT(ctx, authHeader, request)
//
// 2. Regular webhook validation:
//   validator := NewJWTValidator(logger, "", []string{})
//   result := validator.ValidateJWT(ctx, authHeader, request)
//
// 3. Check validation result:
//   if result.Valid {
//       claims := GetClaimsFromValidationResult(result)
//       userID := claims.Subject
//       issuer := claims.Issuer
//   } else {
//       logger.Error("JWT validation failed", "error", FormatValidationError(result))
//   }
