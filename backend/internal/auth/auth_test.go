package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("Password123!")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if strings.Contains(hash, "Password123!") {
		t.Fatal("the hash must not contain the plaintext")
	}
	if !CheckPassword(hash, "Password123!") {
		t.Error("the correct password should verify")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("a wrong password must not verify")
	}
	if CheckPassword("not-a-hash", "Password123!") {
		t.Error("a malformed hash must not verify")
	}
}

func TestHashPasswordRejectsBlank(t *testing.T) {
	if _, err := HashPassword("   "); err == nil {
		t.Error("a blank password should be rejected")
	}
}

func TestHashIsSalted(t *testing.T) {
	a, _ := HashPassword("same-password")
	b, _ := HashPassword("same-password")
	if a == b {
		t.Error("two hashes of the same password must differ (salt)")
	}
}

func TestIssueAndVerify(t *testing.T) {
	issuer := NewIssuer("test-secret", time.Hour)
	token, exp, err := issuer.Issue("user-1", "a@example.com", "admin")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !exp.After(time.Now()) {
		t.Error("expiry should be in the future")
	}

	claims, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("subject = %q, want user-1", claims.Subject)
	}
	if claims.Email != "a@example.com" || claims.Role != "admin" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	token, _, _ := NewIssuer("secret-a", time.Hour).Issue("u", "e@x.com", "employee")
	if _, err := NewIssuer("secret-b", time.Hour).Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("a token signed with another secret must be rejected, got %v", err)
	}
}

func TestVerifyRejectsTamperedToken(t *testing.T) {
	issuer := NewIssuer("secret", time.Hour)
	token, _, _ := issuer.Issue("u", "e@x.com", "employee")
	parts := strings.Split(token, ".")
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	if _, err := issuer.Verify(tampered); err == nil {
		t.Error("a tampered payload must be rejected")
	}
}

func TestVerifyRejectsNoneAlgorithm(t *testing.T) {
	// alg=none with an empty signature: the classic JWT downgrade attack.
	const noneToken = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJhZG1pbiIsInJvbGUiOiJhZG1pbiJ9."
	if _, err := NewIssuer("secret", time.Hour).Verify(noneToken); err == nil {
		t.Error("alg=none must be rejected")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	issuer := NewIssuer("secret", time.Minute)
	base := time.Now()
	issuer.now = func() time.Time { return base }
	token, _, _ := issuer.Issue("u", "e@x.com", "employee")

	issuer.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, err := issuer.Verify(token); !errors.Is(err, ErrExpiredToken) {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
		ok     bool
	}{
		{"Bearer abc.def.ghi", "abc.def.ghi", true},
		{"bearer abc", "abc", true},
		{"BEARER abc", "abc", true},
		{"Bearer ", "", false},
		{"Basic abc", "", false},
		{"abc", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := BearerToken(tc.header)
		if ok != tc.ok || got != tc.want {
			t.Errorf("BearerToken(%q) = (%q, %v), want (%q, %v)", tc.header, got, ok, tc.want, tc.ok)
		}
	}
}
