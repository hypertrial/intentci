package security

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func FuzzV1Redaction(f *testing.F) {
	f.Add([]byte("token"), "before %s after")
	f.Add([]byte{0, 1, 2}, "TOKEN=%s")
	f.Fuzz(func(t *testing.T, secretInput []byte, format string) {
		if len(secretInput)+len(format) > 4096 {
			t.Skip()
		}
		sum := sha256.Sum256(secretInput)
		secret := "intentci-secret-" + hex.EncodeToString(sum[:])
		redactor := NewRedactor([]string{"TOKEN"}, []string{"TOKEN=" + secret})
		content := strings.ReplaceAll(format, "%s", secret)
		redacted := redactor.Redact(content)
		if strings.Contains(redacted, secret) {
			t.Fatalf("secret remained after redaction: %q", redacted)
		}
		if twice := redactor.Redact(redacted); twice != redacted {
			t.Fatalf("redaction is not idempotent: %q != %q", twice, redacted)
		}
	})
}
