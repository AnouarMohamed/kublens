package redact

import (
	"strings"
	"testing"
)

func TestSensitiveTextRedactsCommonSecretShapes(t *testing.T) {
	input := strings.Join([]string{
		"Authorization: Bearer token-abc",
		"password=s3cr3t",
		`api_key:"abcd"`,
		"https://example.test/path?token=secret-123&ok=1",
		"abcdefghijklmnopqrstuvwxyz1234567890",
	}, " ")

	got := SensitiveText(input)
	for _, leaked := range []string{"token-abc", "s3cr3t", "abcd", "secret-123", "abcdefghijklmnopqrstuvwxyz1234567890"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("redacted text leaked %q: %q", leaked, got)
		}
	}
}
