package auth

import (
	"time"
	"errors"
	"strings"
	"net/http"
	"crypto/rand"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/golang-jwt/jwt/v5"
	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error){
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error){
	ok, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error){
		claims := jwt.RegisteredClaims{
			// A usual scenario is to set the expiration time relative to the current time
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "chirpy-access",
			Subject:   userID.String(),
		}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
    token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(tokenSecret), nil
    })
    if err != nil {
        return uuid.UUID{}, err
    }

    claims, ok := token.Claims.(*jwt.RegisteredClaims)
    if !ok {
        return uuid.UUID{}, errors.New("invalid token claims")
    }

    userID, err := uuid.Parse(claims.Subject)
    if err != nil {
        return uuid.UUID{}, err
    }

    return userID, nil
}

func GetBearerToken(headers http.Header) (string, error){
	authHeader := headers.Get("Authorization")
	if authHeader == ""{
		return "", errors.New("No Auth Header Found")
	}

    parts := strings.SplitN(authHeader, " ", 2)
    if len(parts) != 2 || parts[0] != "Bearer" {
        return "", errors.New("invalid Authorization header format")
    }

    return strings.TrimSpace(parts[1]), nil
}

func MakeRefreshToken() string{
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key)

}

func GetAPIKey(headers http.Header) (string, error){
	authHeader := headers.Get("Authorization")
	if authHeader == ""{
		return "", errors.New("No Auth Header Found")
	}

    parts := strings.SplitN(authHeader, " ", 2)
    if len(parts) != 2 || parts[0] != "ApiKey" {
        return "", errors.New("invalid Authorization header format")
    }

    return strings.TrimSpace(parts[1]), nil
}