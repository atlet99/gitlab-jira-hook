package jira

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"
	"os"

	"github.com/atlet99/gitlab-jira-hook/internal/errors"
)

func TestAuthValidator_New(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name            string
		hmacSecret      string
		developmentMode bool
		expectedRequire bool
	}{
		{
			name:            "Production with secret",
			hmacSecret:      "test-secret",
			developmentMode: false,
			expectedRequire: true,
		},
		{
			name:            "Production without secret",
			hmacSecret:      "",
			developmentMode: false,
			expectedRequire: false,
		},
		{
			name:            "Development mode",
			hmacSecret:      "test-secret",
			developmentMode: true,
			expectedRequire: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewAuthValidator(logger, tt.hmacSecret, tt.developmentMode)

			if validator == nil {
				t.Fatal("Expected validator to be created")
			}

			if validator.RequiresAuthentication() != tt.expectedRequire {
				t.Errorf("Expected RequiresAuthentication() = %v, got %v",
					tt.expectedRequire, validator.RequiresAuthentication())
			}
		})
	}
}

func TestAuthValidator_GetAuthenticationType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name         string
		headers      map[string]string
		devMode      bool
		expectedType AuthType
	}{
		{
			name:         "Development mode",
			headers:      map[string]string{},
			devMode:      true,
			expectedType: AuthTypeDevelopment,
		},
		{
			name: "JWT authorization",
			headers: map[string]string{
				"Authorization": "JWT eyJhbGciOiJIUzI1NiJ9.test.sig",
			},
			devMode:      false,
			expectedType: AuthTypeJWT,
		},
		{
			name: "HMAC signature",
			headers: map[string]string{
				"X-Atlassian-Webhook-Signature": "test-signature",
			},
			devMode:      false,
			expectedType: AuthTypeHMAC,
		},
		{
			name: "GitHub style signature",
			headers: map[string]string{
				"X-Hub-Signature-256": "sha256=test-signature",
			},
			devMode:      false,
			expectedType: AuthTypeHMAC,
		},
		{
			name:         "No authentication",
			headers:      map[string]string{},
			devMode:      false,
			expectedType: AuthTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewAuthValidator(logger, "test-secret", tt.devMode)

			req := httptest.NewRequest("POST", "/test", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			authType := validator.GetAuthenticationType(req)
			if authType != tt.expectedType {
				t.Errorf("Expected auth type %s, got %s", tt.expectedType, authType)
			}
		})
	}
}

func TestAuthValidator_ValidateHMACAuth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	secret := "test-webhook-secret"
	validator := NewAuthValidator(logger, secret, false)

	testBody := []byte(`{"test": "data"}`)

	// Generate valid signature
	validSignature := validator.computeHMACSignature(testBody)

	tests := []struct {
		name         string
		headers      map[string]string
		body         []byte
		expectValid  bool
		expectedCode errors.ErrorCode
	}{
		{
			name: "Valid Atlassian signature",
			headers: map[string]string{
				"X-Atlassian-Webhook-Signature": validSignature,
			},
			body:        testBody,
			expectValid: true,
		},
		{
			name: "Valid GitHub-style signature",
			headers: map[string]string{
				"X-Hub-Signature-256": "sha256=" + validSignature,
			},
			body:        testBody,
			expectValid: true,
		},
		{
			name: "Valid generic signature",
			headers: map[string]string{
				"X-Signature": validSignature,
			},
			body:        testBody,
			expectValid: true,
		},
		{
			name: "Invalid signature",
			headers: map[string]string{
				"X-Atlassian-Webhook-Signature": "invalid-signature",
			},
			body:         testBody,
			expectValid:  false,
			expectedCode: errors.ErrCodeInvalidSignature,
		},
		{
			name: "Missing signature",
			headers: map[string]string{
				"Authorization": "Bearer token",
			},
			body:         testBody,
			expectValid:  false,
			expectedCode: errors.ErrCodeInvalidSignature,
		},
		{
			name:         "No secret configured",
			headers:      map[string]string{"X-Signature": "test"},
			body:         testBody,
			expectValid:  false,
			expectedCode: errors.ErrCodeInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use validator with secret for most tests
			testValidator := validator
			if tt.name == "No secret configured" {
				testValidator = NewAuthValidator(logger, "", false)
			}

			req := httptest.NewRequest("POST", "/test", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := testValidator.validateHMACAuth(req, tt.body)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectValid, result.Valid)
			}

			if !tt.expectValid && result.ErrorCode != tt.expectedCode {
				t.Errorf("Expected error code %s, got %s", tt.expectedCode, result.ErrorCode)
			}

			if result.AuthType != AuthTypeHMAC {
				t.Errorf("Expected auth type %s, got %s", AuthTypeHMAC, result.AuthType)
			}
		})
	}
}

func TestAuthValidator_ValidateRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	secret := "test-secret"
	testBody := []byte(`{"test": "data"}`)

	tests := []struct {
		name         string
		validator    *AuthValidator
		headers      map[string]string
		body         []byte
		expectValid  bool
		expectedType AuthType
	}{
		{
			name:         "Development mode bypass",
			validator:    NewAuthValidator(logger, secret, true),
			headers:      map[string]string{},
			body:         testBody,
			expectValid:  true,
			expectedType: AuthTypeDevelopment,
		},
		{
			name:      "JWT authentication",
			validator: NewAuthValidator(logger, secret, false),
			headers: map[string]string{
				"Authorization": "JWT eyJhbGciOiJIUzI1NiJ9.test.sig",
			},
			body:         testBody,
			expectValid:  false, // Will fail JWT validation (no proper key)
			expectedType: AuthTypeJWT,
		},
		{
			name:         "No auth required, no auth provided",
			validator:    NewAuthValidator(logger, "", false),
			headers:      map[string]string{},
			body:         testBody,
			expectValid:  true,
			expectedType: AuthTypeNone,
		},
		{
			name:         "Auth required, no auth provided",
			validator:    NewAuthValidator(logger, secret, false),
			headers:      map[string]string{},
			body:         testBody,
			expectValid:  false,
			expectedType: AuthTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := tt.validator.ValidateRequest(context.Background(), req, tt.body)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectValid, result.Valid)
			}

			if result.AuthType != tt.expectedType {
				t.Errorf("Expected auth type %s, got %s", tt.expectedType, result.AuthType)
			}
		})
	}
}

func TestAuthValidator_IsConnectApp(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewAuthValidator(logger, "test-secret", false)

	tests := []struct {
		name     string
		result   *AuthValidationResult
		expected bool
	}{
		{
			name: "Valid JWT authentication",
			result: &AuthValidationResult{
				Valid:    true,
				AuthType: AuthTypeJWT,
			},
			expected: true,
		},
		{
			name: "Valid HMAC authentication",
			result: &AuthValidationResult{
				Valid:    true,
				AuthType: AuthTypeHMAC,
			},
			expected: false,
		},
		{
			name: "Invalid JWT authentication",
			result: &AuthValidationResult{
				Valid:    false,
				AuthType: AuthTypeJWT,
			},
			expected: false,
		},
		{
			name:     "Nil result",
			result:   nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isConnectApp := validator.IsConnectApp(tt.result)
			if isConnectApp != tt.expected {
				t.Errorf("Expected IsConnectApp=%v, got %v", tt.expected, isConnectApp)
			}
		})
	}
}

func TestAuthValidator_GetUserContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewAuthValidator(logger, "test-secret", false)

	tests := []struct {
		name           string
		result         *AuthValidationResult
		expectedFields []string
	}{
		{
			name: "JWT result with user context",
			result: &AuthValidationResult{
				Valid:           true,
				AuthType:        AuthTypeJWT,
				UserID:          "test-user-123",
				IssuerClientKey: "test-client-key",
				JWTClaims: &JWTClaims{
					IssuedAt:  1234567890,
					ExpiresAt: 1234567890 + 3600,
					Context:   map[string]interface{}{"app": "test"},
				},
			},
			expectedFields: []string{"auth_type", "auth_valid", "user_id", "client_key", "connect_context", "issued_at", "expires_at"},
		},
		{
			name: "HMAC result",
			result: &AuthValidationResult{
				Valid:    true,
				AuthType: AuthTypeHMAC,
			},
			expectedFields: []string{"auth_type", "auth_valid"},
		},
		{
			name:           "Nil result",
			result:         nil,
			expectedFields: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := validator.GetUserContext(tt.result)

			// Check that all expected fields are present
			for _, field := range tt.expectedFields {
				if _, exists := context[field]; !exists {
					t.Errorf("Expected field '%s' to be present in context", field)
				}
			}

			// Check auth_type if result is not nil
			if tt.result != nil {
				if context["auth_type"] != string(tt.result.AuthType) {
					t.Errorf("Expected auth_type=%s, got %v", tt.result.AuthType, context["auth_type"])
				}
				if context["auth_valid"] != tt.result.Valid {
					t.Errorf("Expected auth_valid=%v, got %v", tt.result.Valid, context["auth_valid"])
				}
			}
		})
	}
}

func TestAuthValidator_CreateAuthError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewAuthValidator(logger, "test-secret", false)

	tests := []struct {
		name            string
		result          *AuthValidationResult
		operation       string
		expectNil       bool
		expectedCode    errors.ErrorCode
		expectedMessage string
	}{
		{
			name: "Valid result returns nil",
			result: &AuthValidationResult{
				Valid:    true,
				AuthType: AuthTypeHMAC,
			},
			operation: "test",
			expectNil: true,
		},
		{
			name:      "Nil result returns nil",
			result:    nil,
			operation: "test",
			expectNil: true,
		},
		{
			name: "Invalid signature error",
			result: &AuthValidationResult{
				Valid:        false,
				AuthType:     AuthTypeHMAC,
				ErrorCode:    errors.ErrCodeInvalidSignature,
				ErrorMessage: "Signature validation failed",
			},
			operation:       "webhook processing",
			expectNil:       false,
			expectedCode:    errors.ErrCodeInvalidSignature,
			expectedMessage: "Authentication failed for webhook processing",
		},
		{
			name: "Jira unauthorized error",
			result: &AuthValidationResult{
				Valid:        false,
				AuthType:     AuthTypeJWT,
				ErrorCode:    errors.ErrCodeJiraUnauthorized,
				ErrorMessage: "JWT validation failed",
			},
			operation:       "Connect app request",
			expectNil:       false,
			expectedCode:    errors.ErrCodeJiraUnauthorized,
			expectedMessage: "Authentication failed for Connect app request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authError := validator.CreateAuthError(tt.result, tt.operation)

			if tt.expectNil {
				if authError != nil {
					t.Errorf("Expected nil error, got %v", authError)
				}
				return
			}

			if authError == nil {
				t.Fatal("Expected non-nil error")
			}

			if authError.Code != tt.expectedCode {
				t.Errorf("Expected error code %s, got %s", tt.expectedCode, authError.Code)
			}

			if !strings.Contains(authError.Message, tt.expectedMessage) {
				t.Errorf("Expected message to contain '%s', got: %s", tt.expectedMessage, authError.Message)
			}

			if authError.Category != errors.CategoryClientError {
				t.Errorf("Expected category %s, got %s", errors.CategoryClientError, authError.Category)
			}
		})
	}
}

func TestAuthValidator_ValidateConnectAppPermissions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewAuthValidator(logger, "test-secret", false)

	tests := []struct {
		name           string
		result         *AuthValidationResult
		requiredScopes []string
		expectError    bool
	}{
		{
			name: "Valid Connect app",
			result: &AuthValidationResult{
				Valid:           true,
				AuthType:        AuthTypeJWT,
				IssuerClientKey: "test-client-key",
				JWTClaims: &JWTClaims{
					ScopesString: "READ WRITE",
				},
			},
			requiredScopes: []string{"READ", "WRITE"},
			expectError:    false,
		},
		{
			name: "Not a Connect app (HMAC)",
			result: &AuthValidationResult{
				Valid:    true,
				AuthType: AuthTypeHMAC,
			},
			requiredScopes: []string{"READ"},
			expectError:    true,
		},
		{
			name: "Invalid JWT authentication",
			result: &AuthValidationResult{
				Valid:    false,
				AuthType: AuthTypeJWT,
			},
			requiredScopes: []string{"READ"},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateConnectAppPermissions(tt.result, tt.requiredScopes)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestAuthValidator_ComputeHMACSignature(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	secret := "test-secret-key"
	validator := NewAuthValidator(logger, secret, false)

	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name:     "Empty body",
			body:     []byte{},
			expected: "6b4b8b7a3ed2e2aace659756b7b23b3a8c4cf0b4b8c7a21b5e6e8b7b8b7e8c4a", // Example
		},
		{
			name:     "Simple JSON",
			body:     []byte(`{"test": "data"}`),
			expected: "", // Will be computed dynamically
		},
		{
			name:     "Complex JSON",
			body:     []byte(`{"webhook": "test", "data": {"key": "value", "number": 123}}`),
			expected: "", // Will be computed dynamically
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature := validator.computeHMACSignature(tt.body)

			// Signature should be 64 characters (SHA-256 hex)
			if len(signature) != 64 {
				t.Errorf("Expected signature length 64, got %d", len(signature))
			}

			// Signature should be hex string
			if !isHexString(signature) {
				t.Errorf("Signature is not a valid hex string: %s", signature)
			}

			// Test consistency - same input should produce same output
			signature2 := validator.computeHMACSignature(tt.body)
			if signature != signature2 {
				t.Errorf("HMAC signatures should be consistent, got %s and %s", signature, signature2)
			}

			// Test with different secret produces different signature
			otherValidator := NewAuthValidator(logger, "different-secret", false)
			otherSignature := otherValidator.computeHMACSignature(tt.body)
			if signature == otherSignature && len(tt.body) > 0 {
				t.Errorf("Different secrets should produce different signatures")
			}
		})
	}
}

// Benchmark tests
func BenchmarkAuthValidator_ValidateHMACAuth(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewAuthValidator(logger, "test-secret", false)
	testBody := []byte(`{"test": "data", "number": 123}`)
	signature := validator.computeHMACSignature(testBody)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Atlassian-Webhook-Signature", signature)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.validateHMACAuth(req, testBody)
	}
}

func BenchmarkAuthValidator_ComputeHMACSignature(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewAuthValidator(logger, "test-secret", false)
	testBody := []byte(`{"webhook": "test", "data": {"key": "value"}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.computeHMACSignature(testBody)
	}
}
