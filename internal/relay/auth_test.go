package relay

import (
	"testing"
)

func Test_generate_and_validate_token(t *testing.T) {
	secret := "test-secret-key"
	token := GenerateToken(secret)

	if err := ValidateToken(secret, token); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
}

func Test_reject_wrong_secret(t *testing.T) {
	token := GenerateToken("correct-secret")
	err := ValidateToken("wrong-secret", token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func Test_reject_malformed_token(t *testing.T) {
	err := ValidateToken("secret", "not-a-valid-token")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func Test_reject_empty_token(t *testing.T) {
	err := ValidateToken("secret", "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func Test_token_format(t *testing.T) {
	token := GenerateToken("secret")
	// token should be in format "hmac:timestamp"
	if len(token) < 3 {
		t.Fatalf("token too short: %q", token)
	}
	// should contain exactly one colon separating hmac and timestamp
	colonCount := 0
	for _, c := range token {
		if c == ':' {
			colonCount++
		}
	}
	if colonCount != 1 {
		t.Errorf("expected exactly one colon in token, got %d: %q", colonCount, token)
	}
}
