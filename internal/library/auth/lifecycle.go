package auth

import (
	"time"
)

func NewSession(
	userID int64,
	sessionID string,
	refreshTokenHash string,
	deviceID string,
	devicePlatform string,
	appVersion string,
	now time.Time,
	idleLifetime time.Duration,
	absoluteLifetime time.Duration,
) *Session {
	absoluteExpireAt := now.Add(absoluteLifetime).Unix()
	return &Session{
		UserID:           userID,
		SessionID:        sessionID,
		RefreshTokenHash: refreshTokenHash,
		CreatedAt:        now.Unix(),
		LastRefreshAt:    now.Unix(),
		IdleExpireAt:     CalculateIdleExpireAt(now, idleLifetime, absoluteExpireAt),
		AbsoluteExpireAt: absoluteExpireAt,
		DeviceID:         deviceID,
		DevicePlatform:   devicePlatform,
		AppVersion:       appVersion,
		Status:           SessionStatusActive,
	}
}

func CalculateIdleExpireAt(now time.Time, idleLifetime time.Duration, absoluteExpireAt int64) int64 {
	idleExpireAt := now.Add(idleLifetime).Unix()
	if idleExpireAt > absoluteExpireAt {
		return absoluteExpireAt
	}
	return idleExpireAt
}

func ValidateSessionLifetime(session *Session, now time.Time) error {
	if now.Unix() >= session.AbsoluteExpireAt {
		return lifecycleError(
			401,
			AuthReasonSessionAbsoluteExpired,
			"登录会话已达到最长有效期，请重新登录",
		)
	}
	if now.Unix() >= session.IdleExpireAt {
		return lifecycleError(
			401,
			AuthReasonRefreshIdleExpired,
			"登录会话已因长时间未使用而过期，请重新登录",
		)
	}
	return nil
}

func SessionTTL(session *Session, now time.Time) time.Duration {
	seconds := session.AbsoluteExpireAt - now.Unix()
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
