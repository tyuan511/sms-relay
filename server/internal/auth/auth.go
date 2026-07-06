package auth

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	masterPasswordLength = 32
	passwordCharset      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	TokenTypeUser        = "user"
	TokenTypeDevice      = "device"
)

type Claims struct {
	UserID   string `json:"sub"`
	DeviceID string `json:"device_id,omitempty"`
	Type     string `json:"typ"`
	jwt.RegisteredClaims
}

func GenerateMasterPassword() (string, error) {
	b := make([]byte, masterPasswordLength)
	max := big.NewInt(int64(len(passwordCharset)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = passwordCharset[n.Int64()]
	}
	return string(b), nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func CreateUserToken(userID, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Type:   TokenTypeUser,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func CreateDeviceToken(userID, deviceID, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		DeviceID: deviceID,
		Type:     TokenTypeDevice,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// CreateToken creates a user token for backward compatibility.
func CreateToken(userID, secret string, ttl time.Duration) (string, error) {
	return CreateUserToken(userID, secret, ttl)
}

func ParseToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Type == "" {
		claims.Type = TokenTypeUser
	}
	return claims, nil
}

func (c *Claims) IsUser() bool {
	return c.Type == TokenTypeUser
}

func (c *Claims) IsDevice() bool {
	return c.Type == TokenTypeDevice && c.DeviceID != ""
}
