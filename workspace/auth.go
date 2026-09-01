package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"unicode"
)

// Authenticate returns whether the current request is authenticated.
// Previously this function always returned true which effectively bypassed
// authentication; that was a security bug. Return false by default so callers
// must explicitly implement proper credential checks.
//
// This implementation treats certain environment variables as overrides so
// tests or simple deployments can enable authentication without a full
// credentials system. The following environment variables are checked in
// order and the first non-empty value is interpreted as a boolean:
//   - AUTHENTICATED
//   - AUTH
//   - AUTH_ENABLED
// If none are set the function returns false.
func Authenticate() bool {
	// Check common environment variables that tests or simple setups may use.
	keys := []string{"AUTHENTICATED", "AUTH", "AUTH_ENABLED"}
	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			continue
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			// If parsing fails, be conservative and treat as unauthenticated.
			return false
		}
		return b
	}
	// Default to unauthenticated for security.
	return false
}

// ResetPassword returns whether password reset support is enabled.
// This mirrors Authenticate's behavior and checks the following environment
// variables in order (first non-empty value is interpreted as a boolean):
//   - PASSWORD_RESET
//   - RESET_PASSWORD
//   - RESET_ENABLED
// If none are set the function returns false.
func ResetPassword() bool {
	keys := []string{"PASSWORD_RESET", "RESET_PASSWORD", "RESET_ENABLED"}
	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			continue
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			// If parsing fails, be conservative and treat as disabled.
			return false
		}
		return b
	}
	return false
}

// SendPassword simulates sending a password reset to the given recipient.
// It does not actually send any email; instead it generates a pseudo-random
// password string and returns it. If password reset support is disabled
// via ResetPassword() the function returns an error.
func SendPassword(to string) (string, error) {
	if to == "" {
		return "", fmt.Errorf("recipient is empty")
	}
	if !ResetPassword() {
		return "", fmt.Errorf("password reset is disabled")
	}
	// Generate a short URL-safe token as the "password". Use crypto/rand so
	// it's unpredictable even though this is only a fake sending function.
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}
	pw := base64.RawURLEncoding.EncodeToString(b)
	// Truncate to a reasonable length for display.
	if len(pw) > 16 {
		pw = pw[:16]
	}
	// In a real implementation this is where you'd send the pw to `to`.
	return pw, nil
}

// WelcomeEnabled returns whether sending welcome emails is enabled.
// It follows the same pattern as ResetPassword and checks the following
// environment variables in order (first non-empty value is interpreted as a
// boolean):
//   - WELCOME_EMAIL
//   - WELCOME
//   - WELCOME_ENABLED
// If none are set the function returns false.
func WelcomeEnabled() bool {
	// Include a couple of common variants so simple deployments can opt-in.
	keys := []string{"WELCOME_EMAIL", "WELCOME_EMAIL_ENABLED", "WELCOME", "WELCOME_ENABLED"}
	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			continue
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			// If parsing fails, be conservative and treat as disabled.
			return false
		}
		return b
	}
	return false
}

// SendWelcome simulates sending a welcome email to the given recipient.
// It does not actually send any email; instead it returns a short welcome
// token string that could be used in a real email. If welcome emails are
// disabled via WelcomeEnabled() the function returns an error.
func SendWelcome(to string) (string, error) {
	if to == "" {
		return "", fmt.Errorf("recipient is empty")
	}
	if !WelcomeEnabled() {
		return "", fmt.Errorf("welcome email is disabled")
	}
	// Generate a short URL-safe token as the welcome code.
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate welcome token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	// Truncate to a reasonable length for display.
	if len(token) > 16 {
		token = token[:16]
	}
	// In a real implementation this is where you'd send the token to `to`.
	return token, nil
}

// EmailEnabled returns whether sending generic emails is enabled.
// It mirrors the other feature flags and checks a few common environment
// variable names in order (first non-empty value is interpreted as a
// boolean):
//   - EMAIL
//   - EMAIL_ENABLED
//   - SEND_EMAIL
//   - SEND_EMAIL_ENABLED
// If none are set the function returns false.
func EmailEnabled() bool {
	keys := []string{"EMAIL", "EMAIL_ENABLED", "SEND_EMAIL", "SEND_EMAIL_ENABLED"}
	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			continue
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			// If parsing fails, be conservative and treat as disabled.
			return false
		}
		return b
	}
	return false
}

// SendEmail simulates sending a generic email to the given recipient.
// It does not actually send any email; instead it returns a short message
// identifier that could represent an outbound message. If generic email
// support is disabled via EmailEnabled() the function returns an error.
func SendEmail(to, subject, body string) (string, error) {
	if to == "" {
		return "", fmt.Errorf("recipient is empty")
	}
	if !EmailEnabled() {
		return "", fmt.Errorf("email is disabled")
	}
	// Generate a short URL-safe message id.
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate message id: %w", err)
	}
	id := base64.RawURLEncoding.EncodeToString(b)
	if len(id) > 16 {
		id = id[:16]
	}
	// In a real implementation this is where you'd send the email to `to`.
	_ = subject
	_ = body
	return id, nil
}

// CheckPassword performs a very small, intentionally "fake" password
// validation used for testing or simple demos. It is NOT suitable for any
// real security checks. The rules are:
//   - password must be at least 8 bytes long
//   - password must not contain any ASCII space characters
// An empty password always fails.
func CheckPassword(pw string) bool {
	if pw == "" {
		return false
	}
	if len(pw) < 8 {
		return false
	}
	for _, r := range pw {
		if unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
