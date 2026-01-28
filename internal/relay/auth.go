package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// token validity window.
const _token_validity = 5 * time.Minute

// GenerateToken creates an hmac-sha256 auth token in the format "hmac:timestamp".
func GenerateToken(secret string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := _compute_hmac(secret, ts)
	return mac + ":" + ts
}

// ValidateToken checks an hmac-sha256 auth token against the shared secret.
func ValidateToken(secret, token string) error {
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("malformed token: expected hmac:timestamp")
	}
	mac, tsStr := parts[0], parts[1]

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp in token: %w", err)
	}

	diff := time.Duration(math.Abs(float64(time.Now().Unix()-ts))) * time.Second
	if diff > _token_validity {
		return fmt.Errorf("token expired: age %v exceeds %v", diff, _token_validity)
	}

	expected := _compute_hmac(secret, tsStr)
	if !hmac.Equal([]byte(mac), []byte(expected)) {
		return fmt.Errorf("invalid hmac signature")
	}
	return nil
}

// _compute_hmac generates a hex-encoded hmac-sha256 of the given message.
func _compute_hmac(secret, message string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
