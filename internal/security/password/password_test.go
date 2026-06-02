package password

import "testing"

func TestBCryptHasherHashesAndMatchesPassword(t *testing.T) {
	hasher := NewBCryptHasher()

	hashed, err := hasher.Hash("Password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hashed == "Password123" {
		t.Fatal("password hash must not equal plaintext")
	}
	matches, err := hasher.Matches("Password123", hashed)
	if err != nil {
		t.Fatalf("match password: %v", err)
	}
	if !matches {
		t.Fatal("expected password to match")
	}
	matches, err = hasher.Matches("WrongPassword123", hashed)
	if err != nil {
		t.Fatalf("match wrong password: %v", err)
	}
	if matches {
		t.Fatal("wrong password should not match")
	}
}
