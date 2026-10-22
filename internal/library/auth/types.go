package auth

import "github.com/golang-jwt/jwt/v4"

const (
	AccessTokenType = "access"

	SessionStatusActive  = "active"
	SessionStatusRevoked = "revoked"
)

// Session is the authoritative server-side login session.
type Session struct {
	CredentialVersion         int64  `json:"credentialVersion"`
	UserID                    int64  `json:"userId"`
	SessionID                 string `json:"sessionId"`
	RefreshTokenHash          string `json:"refreshTokenHash"`
	PreviousRefreshTokenHash  string `json:"previousRefreshTokenHash,omitempty"`
	PreviousRefreshGraceUntil int64  `json:"previousRefreshGraceUntil,omitempty"`
	RotationCounter           int64  `json:"rotationCounter"`
	CreatedAt                 int64  `json:"createdAt"`
	LastRefreshAt             int64  `json:"lastRefreshAt"`
	IdleExpireAt              int64  `json:"idleExpireAt"`
	AbsoluteExpireAt          int64  `json:"absoluteExpireAt"`
	DeviceID                  string `json:"deviceId"`
	DevicePlatform            string `json:"devicePlatform"`
	AppVersion                string `json:"appVersion"`
	Status                    string `json:"status"`
}

// TokenPair is returned only at login and refresh boundaries.
type TokenPair struct {
	AccessToken             string `json:"accessToken"`
	AccessExpiresIn         int64  `json:"accessExpiresIn"`
	RefreshToken            string `json:"refreshToken"`
	RefreshIdleExpiresIn    int64  `json:"refreshIdleExpiresIn"`
	RefreshAbsoluteExpireAt int64  `json:"refreshAbsoluteExpireAt"`
	TokenType               string `json:"tokenType"`
	Replayed                bool   `json:"-"`
}

// AccessClaims contains only identity and session references. User profile data
// remains server-side and is loaded after session validation.
type AccessClaims struct {
	CredentialVersion int64  `json:"cv"`
	SessionID         string `json:"sid"`
	Type              string `json:"typ"`
	jwt.RegisteredClaims
}
