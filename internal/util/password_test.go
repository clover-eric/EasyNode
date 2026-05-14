package util

import "testing"

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "password123" {
		t.Fatal("password stored in plaintext")
	}
	if !VerifyPassword("password123", hash) {
		t.Fatal("valid password rejected")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatal("invalid password accepted")
	}
}
