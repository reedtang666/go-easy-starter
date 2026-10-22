package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/reedtang666/go-easy-starter/internal/config"

	"github.com/golang-jwt/jwt/v4"
)

type Clock func() time.Time

type TokenIssuer struct {
	config config.Auth
	now    Clock
}

func NewTokenIssuer(config config.Auth, clock Clock) (*TokenIssuer, error) {
	if config.Issuer == "" || config.Audience == "" {
		return nil, errors.New("auth issuer and audience are required")
	}
	if config.AccessExpires <= 0 {
		return nil, errors.New("auth access expiry must be positive")
	}
	if len(config.AccessSecretKey) < 32 || len(config.RefreshHashKey) < 32 {
		return nil, errors.New("auth signing and hashing keys must be at least 32 bytes")
	}
	if config.AccessSecretKey == config.RefreshHashKey {
		return nil, errors.New("auth signing and hashing keys must differ")
	}
	if clock == nil {
		clock = time.Now
	}
	return &TokenIssuer{config: config, now: clock}, nil
}

func (i *TokenIssuer) IssueAccessToken(userID int64, sessionID string, credentialVersion int64) (string, error) {
	if userID <= 0 || sessionID == "" || credentialVersion <= 0 {
		return "", errors.New("auth user and session are required")
	}
	now := i.now()
	jti, err := randomString(16)
	if err != nil {
		return "", fmt.Errorf("generate access token id: %w", err)
	}
	claims := AccessClaims{
		SessionID:         sessionID,
		CredentialVersion: credentialVersion,
		Type:              AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.config.Issuer,
			Subject:   strconv.FormatInt(userID, 10),
			Audience:  jwt.ClaimStrings{i.config.Audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(i.config.AccessExpires) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(i.config.AccessSecretKey))
}

func (i *TokenIssuer) ParseAccessToken(tokenValue string) (*AccessClaims, error) {
	claims := new(AccessClaims)
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithoutClaimsValidation(),
	)
	token, err := parser.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected access token signing method")
		}
		return []byte(i.config.AccessSecretKey), nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, invalidAccessToken(err)
	}

	now := i.now().Unix()
	if claims.ExpiresAt == nil || now >= claims.ExpiresAt.Unix() {
		return nil, expiredAccessToken()
	}
	if claims.IssuedAt == nil || claims.IssuedAt.Unix() > now {
		return nil, invalidAccessToken(errors.New("invalid issued-at claim"))
	}
	if claims.Issuer != i.config.Issuer ||
		!claims.VerifyAudience(i.config.Audience, true) ||
		claims.Type != AccessTokenType ||
		claims.Subject == "" ||
		claims.SessionID == "" ||
		claims.ID == "" || claims.CredentialVersion <= 0 {
		return nil, invalidAccessToken(errors.New("invalid access token claims"))
	}
	return claims, nil
}

func (i *TokenIssuer) IssueRefreshToken(sessionID string) (string, error) {
	if sessionID == "" || strings.Contains(sessionID, ".") {
		return "", errors.New("invalid session id")
	}
	secret, err := randomString(32)
	if err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return sessionID + "." + secret, nil
}

func (i *TokenIssuer) HashRefreshToken(refreshToken string) string {
	mac := hmac.New(sha256.New, []byte(i.config.RefreshHashKey))
	_, _ = mac.Write([]byte(refreshToken))
	return hex.EncodeToString(mac.Sum(nil))
}

func GenerateSessionID() (string, error) {
	return randomString(24)
}

func ParseRefreshTokenSessionID(refreshToken string) (string, error) {
	parts := strings.Split(refreshToken, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ErrMalformedRefreshToken
	}
	return parts[0], nil
}

func randomString(byteLength int) (string, error) {
	value := make([]byte, byteLength)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
