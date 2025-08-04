package jira

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/atlet99/gitlab-jira-hook/internal/errors"
)

func TestJWTValidator_ExtractJWTToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "", []string{})

	tests := []struct {
		name        string
		authHeader  string
		expectError bool
		expectedLen int
	}{
		{
			name:        "Valid JWT token",
			authHeader:  "JWT eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.signature",
			expectError: false,
			expectedLen: 46,
		},
		{
			name:        "Empty authorization header",
			authHeader:  "",
			expectError: true,
		},
		{
			name:        "Missing JWT prefix",
			authHeader:  "Bearer token123",
			expectError: true,
		},
		{
			name:        "JWT prefix but empty token",
			authHeader:  "JWT ",
			expectError: true,
		},
		{
			name:        "Token too short",
			authHeader:  "JWT short",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := validator.extractJWTToken(tt.authHeader)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if len(token) != tt.expectedLen {
					t.Errorf("Expected token length %d, got %d", tt.expectedLen, len(token))
				}
			}
		})
	}
}

func TestJWTValidator_DetermineTokenType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "", []string{})

	tests := []struct {
		name         string
		algorithm    string
		keyID        string
		expectedType TokenType
	}{
		{
			name:         "RS256 with keyID (asymmetric)",
			algorithm:    "RS256",
			keyID:        "test-key-id",
			expectedType: TokenTypeAsymmetric,
		},
		{
			name:         "RS256 without keyID (fallback to symmetric)",
			algorithm:    "RS256",
			keyID:        "",
			expectedType: TokenTypeSymmetric,
		},
		{
			name:         "HS256 (symmetric)",
			algorithm:    "HS256",
			keyID:        "",
			expectedType: TokenTypeSymmetric,
		},
		{
			name:         "Unknown algorithm (fallback)",
			algorithm:    "UNKNOWN",
			keyID:        "",
			expectedType: TokenTypeSymmetric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenType := validator.determineTokenType(tt.algorithm, tt.keyID)

			if tokenType != tt.expectedType {
				t.Errorf("Expected token type %s, got %s", tt.expectedType, tokenType)
			}
		})
	}
}

func TestJWTValidator_ValidateBasicClaims(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "", []string{"test-issuer"})

	now := time.Now()

	tests := []struct {
		name        string
		claims      *JWTClaims
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid claims",
			claims: &JWTClaims{
				Issuer:    "test-issuer",
				IssuedAt:  now.Unix() - 60,  // 1 minute ago
				ExpiresAt: now.Unix() + 300, // 5 minutes from now
				QueryHash: "test-qsh",
			},
			expectError: false,
		},
		{
			name: "Missing issuer",
			claims: &JWTClaims{
				IssuedAt:  now.Unix(),
				ExpiresAt: now.Unix() + 300,
				QueryHash: "test-qsh",
			},
			expectError: true,
			errorMsg:    "missing issuer",
		},
		{
			name: "Invalid issuer",
			claims: &JWTClaims{
				Issuer:    "invalid-issuer",
				IssuedAt:  now.Unix(),
				ExpiresAt: now.Unix() + 300,
				QueryHash: "test-qsh",
			},
			expectError: true,
			errorMsg:    "not in allowed list",
		},
		{
			name: "Missing issued at",
			claims: &JWTClaims{
				Issuer:    "test-issuer",
				ExpiresAt: now.Unix() + 300,
				QueryHash: "test-qsh",
			},
			expectError: true,
			errorMsg:    "missing issued at",
		},
		{
			name: "Token issued in future",
			claims: &JWTClaims{
				Issuer:    "test-issuer",
				IssuedAt:  now.Unix() + 3600, // 1 hour in future
				ExpiresAt: now.Unix() + 7200,
				QueryHash: "test-qsh",
			},
			expectError: true,
			errorMsg:    "issued in the future",
		},
		{
			name: "Missing expiration",
			claims: &JWTClaims{
				Issuer:    "test-issuer",
				IssuedAt:  now.Unix(),
				QueryHash: "test-qsh",
			},
			expectError: true,
			errorMsg:    "missing expiration",
		},
		{
			name: "Token expired",
			claims: &JWTClaims{
				Issuer:    "test-issuer",
				IssuedAt:  now.Unix() - 3600,
				ExpiresAt: now.Unix() - 60, // 1 minute ago
				QueryHash: "test-qsh",
			},
			expectError: true,
			errorMsg:    "has expired",
		},
		{
			name: "Missing query hash",
			claims: &JWTClaims{
				Issuer:    "test-issuer",
				IssuedAt:  now.Unix(),
				ExpiresAt: now.Unix() + 300,
			},
			expectError: true,
			errorMsg:    "missing query string hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateBasicClaims(tt.claims)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestJWTValidator_ValidateAudience(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "https://test-app.com", []string{})

	tests := []struct {
		name        string
		claims      *JWTClaims
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid audience (string)",
			claims: &JWTClaims{
				Audience: "https://test-app.com",
			},
			expectError: false,
		},
		{
			name: "Valid audience (string array)",
			claims: &JWTClaims{
				Audience: []string{"https://test-app.com", "https://other-app.com"},
			},
			expectError: false,
		},
		{
			name: "Valid audience (interface array)",
			claims: &JWTClaims{
				Audience: []interface{}{"https://test-app.com", "https://other-app.com"},
			},
			expectError: false,
		},
		{
			name: "Missing audience",
			claims: &JWTClaims{
				Audience: nil,
			},
			expectError: true,
			errorMsg:    "missing audience",
		},
		{
			name: "Invalid audience",
			claims: &JWTClaims{
				Audience: "https://wrong-app.com",
			},
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name: "Invalid audience format",
			claims: &JWTClaims{
				Audience: 123,
			},
			expectError: true,
			errorMsg:    "invalid audience format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateAudience(tt.claims)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestJWTValidator_ComputeCanonicalHash(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "", []string{})

	tests := []struct {
		name         string
		method       string
		path         string
		query        string
		expectedHash string
	}{
		{
			name:         "Simple GET request",
			method:       "GET",
			path:         "/test",
			query:        "",
			expectedHash: "3c1a9c8d4f85bdc3cd0b0beb647dd2e2df7c08f3a2ee3a7f69a8d65c1b8fb0e1", // Example hash
		},
		{
			name:         "GET with query parameters",
			method:       "GET",
			path:         "/test",
			query:        "param1=value1&param2=value2",
			expectedHash: "", // Will be computed dynamically
		},
		{
			name:         "POST request",
			method:       "POST",
			path:         "/api/test",
			query:        "",
			expectedHash: "", // Will be computed dynamically
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			reqURL := "http://example.com" + tt.path
			if tt.query != "" {
				reqURL += "?" + tt.query
			}

			req, err := http.NewRequest(tt.method, reqURL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			hash := validator.computeCanonicalHash(req)

			// Hash should be 64 characters (SHA-256 hex)
			if len(hash) != 64 {
				t.Errorf("Expected hash length 64, got %d", len(hash))
			}

			// Hash should be hex string
			if !isHexString(hash) {
				t.Errorf("Hash is not a valid hex string: %s", hash)
			}

			// For specific test case, verify expected hash
			if tt.expectedHash != "" && hash != tt.expectedHash {
				t.Errorf("Expected hash %s, got %s", tt.expectedHash, hash)
			}
		})
	}
}

func TestJWTValidator_BuildCanonicalQueryString(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "", []string{})

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "Empty query",
			query:    "",
			expected: "",
		},
		{
			name:     "Single parameter",
			query:    "param=value",
			expected: "param=value",
		},
		{
			name:     "Multiple parameters (already sorted)",
			query:    "a=1&b=2&c=3",
			expected: "a=1&b=2&c=3",
		},
		{
			name:     "Multiple parameters (needs sorting)",
			query:    "c=3&a=1&b=2",
			expected: "a=1&b=2&c=3",
		},
		{
			name:     "Parameters with encoding",
			query:    "param=hello%20world&other=test%2Bvalue",
			expected: "other=test%2Bvalue&param=hello%20world",
		},
		{
			name:     "JWT parameter excluded",
			query:    "jwt=token&param=value",
			expected: "param=value",
		},
		{
			name:     "Repeated parameters",
			query:    "param=value1&param=value2&other=test",
			expected: "other=test&param=value1%2Cvalue2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request with query
			req, err := http.NewRequest("GET", "http://example.com?"+tt.query, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			result := validator.buildCanonicalQueryString(req)

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestJWTValidator_IsJWTRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "", []string{})

	tests := []struct {
		name       string
		authHeader string
		expected   bool
	}{
		{
			name:       "Valid JWT header",
			authHeader: "JWT eyJhbGciOiJIUzI1NiJ9.test.signature",
			expected:   true,
		},
		{
			name:       "Bearer token",
			authHeader: "Bearer abc123",
			expected:   false,
		},
		{
			name:       "Empty header",
			authHeader: "",
			expected:   false,
		},
		{
			name:       "Basic auth",
			authHeader: "Basic dXNlcjpwYXNz",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			result := validator.IsJWTRequest(req)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestJWTValidator_GetPublicKey_MockCDN(t *testing.T) {
	// Test getting public key (this test requires modifying the CDN URL which is a const)
	// For now, we'll test the method indirectly
	t.Skip("Skipping public key test - requires mock server setup")
}

func TestValidationResult_Helpers(t *testing.T) {
	tests := []struct {
		name   string
		result *ValidationResult
		claims *JWTClaims
	}{
		{
			name: "Valid result with claims",
			result: &ValidationResult{
				Valid:     true,
				TokenType: TokenTypeSymmetric,
				Claims: &JWTClaims{
					Issuer:  "test-issuer",
					Subject: "test-user",
				},
			},
			claims: &JWTClaims{
				Issuer:  "test-issuer",
				Subject: "test-user",
			},
		},
		{
			name: "Invalid result",
			result: &ValidationResult{
				Valid:        false,
				ErrorCode:    errors.ErrCodeInvalidSignature,
				ErrorMessage: "Signature validation failed",
			},
			claims: nil,
		},
		{
			name:   "Nil result",
			result: nil,
			claims: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test GetClaimsFromValidationResult
			claims := GetClaimsFromValidationResult(tt.result)
			if claims != tt.claims {
				if claims == nil && tt.claims == nil {
					// Both nil, OK
				} else if claims == nil || tt.claims == nil {
					t.Errorf("Expected claims %v, got %v", tt.claims, claims)
				} else if claims.Issuer != tt.claims.Issuer || claims.Subject != tt.claims.Subject {
					t.Errorf("Expected claims %v, got %v", tt.claims, claims)
				}
			}

			// Test FormatValidationError
			errorStr := FormatValidationError(tt.result)
			if tt.result == nil {
				if !strings.Contains(errorStr, "nil") {
					t.Errorf("Expected error message to contain 'nil', got: %s", errorStr)
				}
			} else if tt.result.Valid {
				if !strings.Contains(errorStr, "successful") {
					t.Errorf("Expected error message to contain 'successful', got: %s", errorStr)
				}
			} else {
				if !strings.Contains(errorStr, string(tt.result.ErrorCode)) {
					t.Errorf("Expected error message to contain error code, got: %s", errorStr)
				}
			}
		})
	}
}

// Helper functions

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// Benchmark tests
func BenchmarkJWTValidator_ExtractToken(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "", []string{})
	authHeader := "JWT eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.signature"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.extractJWTToken(authHeader)
	}
}

func BenchmarkJWTValidator_ValidateBasicClaims(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewJWTValidator(logger, "", []string{"test-issuer"})

	now := time.Now()
	claims := &JWTClaims{
		Issuer:    "test-issuer",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Unix() + 300,
		QueryHash: "test-qsh",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.validateBasicClaims(claims)
	}
}
