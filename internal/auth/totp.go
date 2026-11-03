package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// GenerateTOTPSecret creates a secure, random 20-byte Base32-encoded secret for RFC 6238 TOTP
func GenerateTOTPSecret() (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// GenerateTOTPURI generates the otpauth:// URI for QR code generation
func GenerateTOTPURI(secret, accountName, issuer string) string {
	if issuer == "" {
		issuer = "AOSphere"
	}
	label := fmt.Sprintf("%s:%s", issuer, accountName)
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")

	return fmt.Sprintf("otpauth://totp/%s?%s", url.PathEscape(label), v.Encode())
}

// ComputeTOTPCode calculates the 6-digit TOTP code for a given secret at time t (RFC 6238)
func ComputeTOTPCode(secret string, t time.Time) (string, error) {
	cleanSecret := strings.ToUpper(strings.TrimSpace(secret))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleanSecret)
	if err != nil {
		// Try with standard padding if needed
		key, err = base32.StdEncoding.DecodeString(cleanSecret)
		if err != nil {
			return "", fmt.Errorf("invalid base32 secret: %w", err)
		}
	}

	counter := uint64(t.Unix() / 30)
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(counterBytes)
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	binaryCode := (uint32(hash[offset]&0x7f) << 24) |
		(uint32(hash[offset+1]&0xff) << 16) |
		(uint32(hash[offset+2]&0xff) << 8) |
		uint32(hash[offset+3]&0xff)

	otp := binaryCode % 1000000
	return fmt.Sprintf("%06d", otp), nil
}

// VerifyTOTPCode checks whether code matches the secret within a +/- 1 step (30s) window
func VerifyTOTPCode(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}

	now := time.Now()
	// Check current time, -30s, and +30s to tolerate slight clock skew
	steps := []time.Time{
		now,
		now.Add(-30 * time.Second),
		now.Add(30 * time.Second),
	}

	for _, stepTime := range steps {
		expected, err := ComputeTOTPCode(secret, stepTime)
		if err == nil && expected == code {
			return true
		}
	}
	return false
}

// GenerateRecoveryCodes produces 8 cryptographically secure single-use backup codes (format: XXXX-XXXX)
func GenerateRecoveryCodes(count int) ([]string, error) {
	if count <= 0 {
		count = 8
	}
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Crockford-style base32 to prevent visual confusion
	codes := make([]string, count)

	for i := 0; i < count; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		var sb strings.Builder
		for j, val := range b {
			if j == 4 {
				sb.WriteByte('-')
			}
			sb.WriteByte(charset[int(val)%len(charset)])
		}
		codes[i] = sb.String()
	}

	return codes, nil
}

