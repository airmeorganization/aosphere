package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"unicode"
)

// ValidatePasswordComplexity enforces enterprise SOC2 Type II password requirements
func ValidatePasswordComplexity(pwd string) error {
	if len(pwd) < 10 {
		return errors.New("password must be at least 10 characters long")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range pwd {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter (A-Z)")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter (a-z)")
	}
	if !hasDigit {
		return errors.New("password must contain at least one number (0-9)")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character (!@#$%^&*)")
	}

	return nil
}

// GenerateSecureToken creates a cryptographically secure random token (e.g. for invites or password resets)
func GenerateSecureToken(byteLength int) (rawToken string, tokenHash string, err error) {
	if byteLength <= 0 {
		byteLength = 32
	}
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	rawToken = hex.EncodeToString(bytes)
	tokenHash = HashToken(rawToken)
	return rawToken, tokenHash, nil
}

