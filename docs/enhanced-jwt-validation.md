# Enhanced JWT Validation for Jira Connect Apps

The GitLab ↔ Jira Hook service now includes comprehensive JWT validation support for Jira Connect apps, implementing the official Atlassian JWT specification.

## Overview

This enhancement adds production-ready JWT validation for Jira Cloud Connect apps, supporting both:

- **Asymmetric JWT (RS256)** - For lifecycle callbacks (install/uninstall) with public key validation
- **Symmetric JWT (HS256)** - For regular webhook communications with shared secret
- **HMAC-SHA256** - Legacy webhook signature validation (backward compatible)

## Implementation

### Key Components

#### 1. JWTValidator (`internal/jira/jwt_validator.go`)
- Handles JWT token parsing and validation
- Supports both RS256 (asymmetric) and HS256 (symmetric) algorithms
- Fetches and caches public keys from Atlassian CDN
- Validates all required JWT claims (iss, iat, exp, qsh, aud)
- Implements canonical query string hash validation
- Provides comprehensive error handling with structured error types

#### 2. AuthValidator (`internal/jira/auth_validator.go`)
- Unified authentication validation for all webhook types
- Automatically detects authentication type (JWT, HMAC, or none)
- Supports development mode bypass for testing
- Provides user context extraction and Connect app detection
- Creates structured authentication errors

#### 3. Enhanced WebhookHandler (`internal/jira/webhook_handler.go`)
- Integrated with the new authentication system
- Uses type-safe context keys for request context
- Logs detailed authentication information
- Supports both Connect apps and traditional webhooks

### Supported JWT Features

#### Asymmetric JWT (RS256) - Lifecycle Callbacks
- **Public Key Retrieval**: Fetches keys from `https://connect-install-keys.atlassian.com/`
- **Key Caching**: In-memory caching with configurable TTL
- **Signature Verification**: RSA signature verification with public key
- **Claims Validation**: Full validation of all required and optional claims
- **Audience Validation**: Validates `aud` claim against expected app base URL

#### Symmetric JWT (HS256) - Regular Webhooks
- **Shared Secret**: Uses secret established during app installation
- **HMAC Verification**: HS256 signature validation
- **Query String Hash**: Validates `qsh` claim against canonical request
- **Context JWTs**: Supports context JWTs with fixed `qsh` value

### Security Features

#### JWT Token Validation
- **Algorithm Verification**: Ensures correct signing algorithm (prevents `alg: none` attacks)
- **Timing Validation**: Validates `iat` and `exp` claims with clock skew tolerance
- **Issuer Validation**: Configurable allowed issuer list
- **Audience Validation**: Validates intended recipient
- **Query Hash Validation**: Prevents URL tampering

#### HMAC Signature Validation (Backward Compatible)
- **Multiple Headers**: Supports `X-Atlassian-Webhook-Signature`, `X-Hub-Signature-256`, `X-Signature`
- **Constant-Time Comparison**: Prevents timing attacks using `hmac.Equal`
- **Flexible Formats**: Handles various signature formats (with/without `sha256=` prefix)

## Configuration

### Environment Variables

```bash
# Required for JWT validation
JIRA_WEBHOOK_SECRET=your-shared-secret-from-installation

# Optional: Development mode (skips authentication)
DEBUG_MODE=false

# Optional: Configure allowed issuers and audience
# (Set programmatically via ConfigureJWTValidation method)
```

### Programmatic Configuration

```go
// Configure JWT validation for Connect apps
webhookHandler.ConfigureJWTValidation(
    "https://your-app.com",              // Expected audience
    []string{"known-client-key-1", "known-client-key-2"}, // Allowed issuers
)
```

## Usage Examples

### Basic Webhook Validation

```go
// The webhook handler automatically detects and validates authentication
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
    // Authentication is automatically validated before reaching this point
    // User context is available in the request context if needed
    
    userID := r.Context().Value(jiraUserIDKey)
    clientKey := r.Context().Value(jiraClientKeyKey)
}
```

### Manual Authentication Validation

```go
// Create validator
validator := NewAuthValidator(logger, hmacSecret, false)

// For Connect apps, configure JWT validation
validator = validator.WithJWTValidator("https://your-app.com", allowedIssuers)

// Validate request
result := validator.ValidateRequest(ctx, request, body)
if !result.Valid {
    authError := validator.CreateAuthError(result, "operation")
    return authError
}

// Check if it's a Connect app
if validator.IsConnectApp(result) {
    userID := result.UserID
    clientKey := result.IssuerClientKey
    // Handle Connect app specific logic
}
```

### Different Authentication Types

```go
authType := validator.GetAuthenticationType(request)
switch authType {
case AuthTypeJWT:
    // Jira Connect app with JWT token
case AuthTypeHMAC:
    // Traditional webhook with HMAC signature
case AuthTypeDevelopment:
    // Development mode bypass
case AuthTypeNone:
    // No authentication provided
}
```

## Error Handling

The system provides structured error handling with specific error codes:

- `INVALID_SIGNATURE` - JWT signature or HMAC validation failed
- `JIRA_UNAUTHORIZED` - JWT authentication failed
- `VALIDATION_FAILED` - Claims validation failed
- `INVALID_WEBHOOK` - Malformed webhook or JWT token

Each error includes:
- Detailed error message for debugging
- User-friendly message for client responses
- Request context (method, path, headers)
- Authentication type and failure reason

## Testing

Comprehensive test suite covers:

- JWT token extraction and parsing
- Signature verification for both RS256 and HS256
- Claims validation (timing, issuer, audience)
- Canonical query string computation
- HMAC signature validation
- Authentication type detection
- Error handling scenarios

Run tests:
```bash
go test ./internal/jira/ -v -run "JWT|Auth"
```

## Integration with Existing Code

The enhanced JWT validation is fully backward compatible:

1. **Existing HMAC webhooks** continue to work unchanged
2. **Development mode** can bypass authentication for testing
3. **Gradual migration** from HMAC to JWT is supported
4. **Error handling** is enhanced but maintains existing interfaces

## Security Considerations

### Production Deployment
- Always configure `JIRA_WEBHOOK_SECRET` in production
- Use HTTPS for all webhook endpoints
- Validate audience claims for Connect apps
- Monitor authentication failures for security threats
- Regularly rotate webhook secrets

### Key Management
- Public keys are automatically fetched and cached from Atlassian CDN
- Shared secrets should be stored securely (environment variables, secret management)
- Key caching reduces CDN requests but should have reasonable TTL

### Performance
- JWT validation adds minimal latency (~1-2ms per request)
- Public key caching reduces external requests
- HMAC validation is very fast (microseconds)
- No external dependencies for HMAC validation

## Monitoring and Observability

The system provides detailed logging and metrics:

### Authentication Logs
```json
{
  "level": "INFO",
  "msg": "Authentication successful",
  "auth_type": "jwt",
  "auth_context": {
    "user_id": "user-123",
    "client_key": "connect-app-key",
    "issued_at": 1234567890,
    "expires_at": 1234571490
  }
}
```

### Error Logs
```json
{
  "level": "WARN", 
  "msg": "Authentication failed",
  "error": "JWT signature verification failed",
  "auth_type": "jwt",
  "remote_addr": "1.2.3.4"
}
```

## Future Enhancements

Planned improvements include:

- **Shared Secret Management**: Integration with secret management systems
- **JWT Caching**: Cache validated JWTs to reduce validation overhead
- **Advanced Analytics**: Authentication pattern analysis
- **Connect App Permissions**: Scope-based permission validation
- **Webhook Replay Protection**: Nonce-based replay prevention

## References

- [Atlassian Connect JWT Documentation](https://developer.atlassian.com/cloud/jira/platform/understanding-jwt-for-connect-apps/)
- [Jira Cloud Platform Webhooks](https://developer.atlassian.com/cloud/jira/platform/webhooks/)
- [JWT RFC 7519](https://tools.ietf.org/html/rfc7519)
- [HMAC RFC 2104](https://tools.ietf.org/html/rfc2104)