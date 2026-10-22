package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSessionNotFound = errors.New("auth session not found")
	ErrSessionReplaced = errors.New("auth session replaced")
	ErrRefreshInvalid  = errors.New("auth refresh token invalid")
	ErrRefreshReused   = errors.New("auth refresh token reused")
	ErrDeviceMismatch  = errors.New("auth refresh device mismatch")
	ErrIdleExpired     = errors.New("auth refresh idle expiry reached")
	ErrAbsoluteExpired = errors.New("auth absolute expiry reached")
)

type RotateInput struct {
	SessionID                 string
	UserID                    int64
	PresentedTokenHash        string
	NewRefreshTokenHash       string
	DeviceID                  string
	AppVersion                string
	NowUnix                   int64
	IdleExpireAt              int64
	PreviousRefreshGraceUntil int64
	TTL                       time.Duration
	ReplayPayload             string
	ReplayTTLSeconds          int64
}

type RotateResult struct {
	Session       *Session
	ReplayPayload string
}

type Store interface {
	CreateOrReplace(ctx context.Context, session *Session, ttl time.Duration) (replacedSessionID string, err error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	GetUserSessionID(ctx context.Context, userID int64) (string, error)
	Rotate(ctx context.Context, input RotateInput) (*RotateResult, error)
	RevokeSession(ctx context.Context, userID int64, sessionID string) error
	RevokeUser(ctx context.Context, userID int64) (sessionID string, err error)
}
