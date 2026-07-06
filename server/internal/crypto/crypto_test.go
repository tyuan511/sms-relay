package crypto

import "testing"

func TestPasswordFingerprintUsesPepper(t *testing.T) {
	a := PasswordFingerprint("secret-password", "pepper-a")
	b := PasswordFingerprint("secret-password", "pepper-b")
	if a == b {
		t.Fatal("expected different peppers to produce different fingerprints")
	}
	if a == LegacyPasswordFingerprint("secret-password") {
		t.Fatal("expected HMAC fingerprint to differ from legacy SHA-256")
	}
}

func TestPasswordFingerprintDeterministic(t *testing.T) {
	password := "abc123XYZ"
	pepper := "server-pepper"
	first := PasswordFingerprint(password, pepper)
	second := PasswordFingerprint(password, pepper)
	if first != second {
		t.Fatalf("expected deterministic fingerprint, got %q and %q", first, second)
	}
}
