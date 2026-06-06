package api

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/ah2909/crypto-alert-engine/config"
)

func authServiceURL() string {
	return config.GetEnv("AUTH_SERVICE_URL", "http://auth-service:8086")
}

// timeout 10s
var jwksClient = &http.Client{Timeout: 10 * time.Second}

// jsonWebKey is a single RSA key from a JWKS document.
type jsonWebKey struct {
	Kty string `json:"kty"`
	N   string `json:"n"` // modulus, base64url
	E   string `json:"e"` // exponent, base64url
	Kid string `json:"kid"`
}

type jsonWebKeySet struct {
	Keys []jsonWebKey `json:"keys"`
}

// Cached public key. Unlike sync.Once, we cache only on success so a transient
// JWKS failure is retried on the next request (matching the Node version, which
// leaves cachedPublicKey null when the fetch throws).
var (
	keyMu     sync.RWMutex
	cachedKey *rsa.PublicKey
)

func getPublicKey() (*rsa.PublicKey, error) {
	keyMu.RLock()
	k := cachedKey
	keyMu.RUnlock()
	if k != nil {
		return k, nil
	}

	keyMu.Lock()
	defer keyMu.Unlock()
	if cachedKey != nil { // another goroutine may have filled it
		return cachedKey, nil
	}

	fetched, err := fetchPublicKey()
	if err != nil {
		return nil, err
	}
	cachedKey = fetched
	return cachedKey, nil
}

func fetchPublicKey() (*rsa.PublicKey, error) {
	url := strings.TrimRight(authServiceURL(), "/") + "/.well-known/jwks.json"

	res, err := jwksClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("JWKS fetch failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS fetch failed: %d", res.StatusCode)
	}

	var set jsonWebKeySet
	if err := json.NewDecoder(res.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("JWKS decode failed: %w", err)
	}
	if len(set.Keys) == 0 {
		return nil, errors.New("no keys found in JWKS response")
	}
	return jwkToRSAPublicKey(set.Keys[0])
}

// jwkToRSAPublicKey builds an *rsa.PublicKey from an RSA JWK's modulus/exponent.
func jwkToRSAPublicKey(k jsonWebKey) (*rsa.PublicKey, error) {
	if k.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported JWK key type %q (want RSA)", k.Kty)
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("invalid JWK modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("invalid JWK exponent: %w", err)
	}

	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

// AuthMiddleware verifies the RS256 bearer token on each request and stores the
// decoded claims in the Gin context under "user". Apply it to protected routes:
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			respondError(c, http.StatusUnauthorized, "Authentication error: No token provided")
			c.Abort()
			return
		}

		key, err := getPublicKey()
		if err != nil {
			respondError(c, http.StatusServiceUnavailable, "Authentication error: unable to load verification key")
			c.Abort()
			return
		}

		claims := jwt.MapClaims{}
		_, err = jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
			return key, nil
		}, jwt.WithValidMethods([]string{"RS256"}))
		if err != nil {
			respondError(c, http.StatusUnauthorized, "Authentication error: Invalid or expired token")
			c.Abort()
			return
		}

		uid, ok := userIDFromClaims(claims)
		if !ok {
			respondError(c, http.StatusUnauthorized, "Authentication error: token has no usable user id")
			c.Abort()
			return
		}

		c.Set("user", claims)  // full claims, if a handler needs more
		c.Set(ctxUserID, uid)  // authenticated user id — use this for ownership
		c.Next()
	}
}

const ctxUserID = "userID"

func authUserID(c *gin.Context) uint {
	if v, ok := c.Get(ctxUserID); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

func userIDFromClaims(claims jwt.MapClaims) (uint, bool) {
	switch t := claims["id"].(type) {
	case float64:
		if t > 0 {
			return uint(t), true
		}
	case json.Number:
		if n, err := strconv.ParseUint(t.String(), 10, 64); err == nil && n > 0 {
			return uint(n), true
		}
	}
	return 0, false
}

func bearerToken(c *gin.Context) string {
	const prefix = "Bearer "
	h := c.GetHeader("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return ""
}
