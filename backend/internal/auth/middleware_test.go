package auth

import (
	"context"
	"testing"
)

func TestAuthenticatorVerifiesStaticTokenFromDigest(t *testing.T) {
	authenticator := NewAuthenticator(Config{
		Enabled: true,
		Tokens: []Token{
			{Token: "viewer-secret", User: "viewer", Role: "viewer"},
			{Token: "operator-secret", User: "operator", Role: "operator"},
		},
	})

	principal, err := authenticator.VerifyToken(context.Background(), "operator-secret")
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if principal.User != "operator" || principal.Role != RoleOperator || principal.Provider != "static" {
		t.Fatalf("unexpected principal: %+v", principal)
	}
}

func TestAuthenticatorRejectsUnknownStaticToken(t *testing.T) {
	authenticator := NewAuthenticator(Config{
		Enabled: true,
		Tokens:  []Token{{Token: "viewer-secret", User: "viewer", Role: "viewer"}},
	})

	if _, err := authenticator.VerifyToken(context.Background(), "viewer-secret-extra"); err == nil {
		t.Fatal("expected unknown static token to be rejected")
	}
}
