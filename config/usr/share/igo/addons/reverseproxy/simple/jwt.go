package simple

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	secretKey = "b8f3e2c1a4d5e6f7g8h9i0j1k2l3m4n5o6p7q8r9s0t1u2v3w4x5y6z7"
)

type JWTUser struct {
	IsValid bool `json:"valid"`
	// HasSecret bool   `json:"secret"`
	RouteId string `json:"routeId"`
	Host    string `json:"host"`
	Domain  string `json:"domain"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	jwt.RegisteredClaims
}

func (c JWTUser) ToString() string {
	return fmt.Sprintf("[valid:(%t), host:(%s), domain:(%s), name:(%s), email:(%s)]",
		c.IsValid, c.Host, c.Domain, c.Name, c.Email)
}

func Encode(c JWTUser) (string, error) {
	// Define your secret key
	secretKey := []byte(secretKey)

	// Create a new token object, specifying signing method and claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"domain": c.Domain,
		"name":   c.Name,
		"email":  c.Email,
		"exp":    time.Now().Add(time.Hour * 1).Unix(), // Expires in 1 hour
	})

	// Sign and get the complete encoded token as a string
	signedString, err := token.SignedString(secretKey)
	if err != nil {
		panic(err)
	}
	return signedString, nil
}

func Decode(tokenString string) (*JWTUser, error) {
	claims := &JWTUser{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}
