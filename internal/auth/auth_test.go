package auth

import (
    "testing"
	"net/http"
)

func TestHashPassword(t *testing.T) {
    hash, err := HashPassword("mypassword")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if hash == "" {
        t.Fatal("expected a non-empty hash")
    }
}

func TestCheckPasswordHash(t *testing.T) {
    password := "mypassword"
    hash, err := HashPassword(password)
    if err != nil {
        t.Fatalf("couldn't hash password: %v", err)
    }

    ok, err := CheckPasswordHash(password, hash)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !ok {
        t.Fatal("expected password to match hash")
    }
}

func TestCheckPasswordHashWrongPassword(t *testing.T) {
    hash, err := HashPassword("mypassword")
    if err != nil {
        t.Fatalf("couldn't hash password: %v", err)
    }

    ok, err := CheckPasswordHash("wrongpassword", hash)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if ok {
        t.Fatal("expected password to not match hash")
    }
}

func TestGetBearerToken(t *testing.T) {
    // valid token
    headers := http.Header{}
    headers.Set("Authorization", "Bearer mytoken123")
    token, err := GetBearerToken(headers)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if token != "mytoken123" {
        t.Fatalf("expected 'mytoken123', got '%s'", token)
    }

    // missing header
    emptyHeaders := http.Header{}
    _, err = GetBearerToken(emptyHeaders)
    if err == nil {
        t.Fatal("expected error for missing Authorization header")
    }

    // wrong scheme (not Bearer)
    badHeaders := http.Header{}
    badHeaders.Set("Authorization", "Basic mytoken123")
    _, err = GetBearerToken(badHeaders)
    if err == nil {
        t.Fatal("expected error for non-Bearer Authorization header")
    }

    // extra whitespace
    spaceyHeaders := http.Header{}
    spaceyHeaders.Set("Authorization", "Bearer   mytoken123")
    token, err = GetBearerToken(spaceyHeaders)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if token != "mytoken123" {
        t.Fatalf("expected 'mytoken123', got '%s'", token)
    }
}