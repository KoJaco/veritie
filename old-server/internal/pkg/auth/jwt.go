// auth/jwt.go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"schma.ai/internal/pkg/logger"
)

type WSClaims struct {
	jwt.RegisteredClaims
	Typ string `json:"typ"`
	Sid string `json:"sid"`
}

// parseWsJWT verifies HS256, issuer, exp (with leeway), and required claims.
func ParseWsJWT(tokenStr string, expectedIss string) (*WSClaims, error) {
	   keyBytes, err := LoadSigningKeyFromEnv("JWT_SIGNING_KEY")
    if err != nil { return nil, err }

    parser := jwt.NewParser(
        jwt.WithValidMethods([]string{"HS256"}),
        jwt.WithLeeway(30*time.Second),
    )

	logger.ServiceDebugf("AUTH", "tokenStr=%s, expectedIss=%s", tokenStr, expectedIss)


	claims := &WSClaims{}
	
	_, err = parser.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return keyBytes, nil
	})

	logger.ServiceDebugf("AUTH", "claims=%+v, err=%v", claims, err)
	

	if err != nil {
		return nil, err
	}
	if claims.Typ != "ws-client" || claims.Issuer != expectedIss {
		return nil, fmt.Errorf("bad typ/iss")
	}
	if claims.Subject == "" || claims.Sid == "" {
		return nil, fmt.Errorf("missing sub/sid")
	}
	return claims, nil
}
