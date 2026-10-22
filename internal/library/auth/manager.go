package auth

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/reedtang666/go-easy-starter/internal/config"
)

type Device struct {
	ID         string
	Platform   string
	AppVersion string
}

type Manager struct {
	config config.Auth
	store  Store
	issuer *TokenIssuer
	now    Clock
}

func NewManager(config config.Auth, store Store, clock Clock) (*Manager, error) {
	if store == nil {
		return nil, errors.New("auth store is required")
	}
	if config.RefreshIdleExpires <= 0 ||
		config.RefreshAbsoluteExpires <= 0 ||
		config.RefreshReuseGrace < 0 {
		return nil, errors.New("auth refresh lifetimes are invalid")
	}
	if config.RefreshIdleExpires > config.RefreshAbsoluteExpires {
		return nil, errors.New("auth idle expiry cannot exceed absolute expiry")
	}
	if clock == nil {
		clock = time.Now
	}
	issuer, err := NewTokenIssuer(config, clock)
	if err != nil {
		return nil, err
	}
	return &Manager{config: config, store: store, issuer: issuer, now: clock}, nil
}

func (m *Manager) Login(
	ctx context.Context,
	userID int64,
	credentialVersion int64,
	device Device,
) (*TokenPair, string, error) {
	sessionID, err := GenerateSessionID()
	if err != nil {
		return nil, "", err
	}
	refreshToken, err := m.issuer.IssueRefreshToken(sessionID)
	if err != nil {
		return nil, "", err
	}
	now := m.now()
	session := NewSession(
		userID,
		sessionID,
		m.issuer.HashRefreshToken(refreshToken),
		device.ID,
		device.Platform,
		device.AppVersion,
		now,
		time.Duration(m.config.RefreshIdleExpires)*time.Second,
		time.Duration(m.config.RefreshAbsoluteExpires)*time.Second,
	)
	session.CredentialVersion = credentialVersion
	replacedSessionID, err := m.store.CreateOrReplace(ctx, session, SessionTTL(session, now))
	if err != nil {
		return nil, "", infrastructureError("create session", err)
	}
	pair, err := m.issuePair(userID, session, refreshToken, now)
	if err != nil {
		_ = m.store.RevokeSession(ctx, userID, sessionID)
		return nil, "", err
	}
	return pair, replacedSessionID, nil
}

func (m *Manager) ValidateAccess(ctx context.Context, accessToken string) (*Session, error) {
	session, _, err := m.ValidateAccessWithClaims(ctx, accessToken)
	return session, err
}

func (m *Manager) ValidateAccessWithClaims(
	ctx context.Context,
	accessToken string,
) (*Session, *AccessClaims, error) {
	claims, err := m.issuer.ParseAccessToken(accessToken)
	if err != nil {
		return nil, nil, err
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID <= 0 {
		return nil, nil, invalidAccessToken(err)
	}
	currentSessionID, err := m.store.GetUserSessionID(ctx, userID)
	if err != nil {
		return nil, nil, infrastructureError("get user session", err)
	}
	if currentSessionID != "" && currentSessionID != claims.SessionID {
		return nil, nil, sessionReplacedError()
	}
	session, err := m.store.GetSession(ctx, claims.SessionID)
	if err != nil {
		return nil, nil, infrastructureError("get session", err)
	}
	if session == nil || session.UserID != userID || session.Status != SessionStatusActive || session.CredentialVersion != claims.CredentialVersion {
		return nil, nil, invalidAccessToken(ErrSessionNotFound)
	}
	if err = ValidateSessionLifetime(session, m.now()); err != nil {
		return nil, nil, err
	}
	if currentSessionID == "" {
		return nil, nil, invalidAccessToken(ErrSessionNotFound)
	}
	return session, claims, nil
}

func (m *Manager) ValidateSession(ctx context.Context, userID int64, sessionID string) (*Session, error) {
	if userID <= 0 || sessionID == "" {
		return nil, invalidAccessToken(ErrSessionNotFound)
	}
	currentSessionID, err := m.store.GetUserSessionID(ctx, userID)
	if err != nil {
		return nil, infrastructureError("get user session", err)
	}
	if currentSessionID == "" {
		return nil, invalidAccessToken(ErrSessionNotFound)
	}
	if currentSessionID != sessionID {
		return nil, sessionReplacedError()
	}
	session, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, infrastructureError("get session", err)
	}
	if session == nil || session.UserID != userID || session.Status != SessionStatusActive {
		return nil, invalidAccessToken(ErrSessionNotFound)
	}
	if err = ValidateSessionLifetime(session, m.now()); err != nil {
		return nil, err
	}
	return session, nil
}

func (m *Manager) ParseAccessToken(accessToken string) (*AccessClaims, error) {
	return m.issuer.ParseAccessToken(accessToken)
}

func (m *Manager) Refresh(
	ctx context.Context,
	refreshToken string,
	deviceID string,
) (*TokenPair, error) {
	return m.RefreshWithMetadata(ctx, refreshToken, deviceID, "")
}

func (m *Manager) RefreshWithMetadata(
	ctx context.Context,
	refreshToken string,
	deviceID string,
	appVersion string,
) (*TokenPair, error) {
	sessionID, err := ParseRefreshTokenSessionID(refreshToken)
	if err != nil {
		return nil, refreshInvalidError(err)
	}
	newRefreshToken, err := m.issuer.IssueRefreshToken(sessionID)
	if err != nil {
		return nil, err
	}
	now := m.now()
	input := RotateInput{
		SessionID:                 sessionID,
		PresentedTokenHash:        m.issuer.HashRefreshToken(refreshToken),
		NewRefreshTokenHash:       m.issuer.HashRefreshToken(newRefreshToken),
		DeviceID:                  deviceID,
		AppVersion:                appVersion,
		NowUnix:                   now.Unix(),
		PreviousRefreshGraceUntil: now.Add(time.Duration(m.config.RefreshReuseGrace) * time.Second).Unix(),
	}

	session, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, infrastructureError("get refresh session", err)
	}
	if session == nil {
		return nil, refreshInvalidError(ErrSessionNotFound)
	}
	if err = ValidateSessionLifetime(session, now); err != nil {
		return nil, err
	}
	input.IdleExpireAt = CalculateIdleExpireAt(
		now,
		time.Duration(m.config.RefreshIdleExpires)*time.Second,
		session.AbsoluteExpireAt,
	)
	input.UserID = session.UserID
	input.TTL = SessionTTL(session, now)
	proposedSession := *session
	proposedSession.IdleExpireAt = input.IdleExpireAt
	pair, err := m.issuePair(session.UserID, &proposedSession, newRefreshToken, now)
	if err != nil {
		return nil, err
	}
	input.ReplayPayload, err = sealRefreshReplay(m.config.RefreshHashKey, pair)
	if err != nil {
		return nil, infrastructureError("seal refresh replay", err)
	}
	input.ReplayTTLSeconds = m.config.RefreshReuseGrace

	rotation, err := m.store.Rotate(ctx, input)
	if err != nil {
		return nil, m.mapRotateError(err)
	}
	if rotation.ReplayPayload != "" {
		replayed, replayErr := openRefreshReplay(m.config.RefreshHashKey, rotation.ReplayPayload)
		if replayErr != nil {
			return nil, infrastructureError("open refresh replay", replayErr)
		}
		replayed.Replayed = true
		return replayed, nil
	}
	if rotation.Session == nil {
		return nil, infrastructureError("rotate refresh token", errors.New("missing rotated session"))
	}
	return pair, nil
}

func (m *Manager) Logout(ctx context.Context, userID int64, sessionID string) error {
	if userID <= 0 || sessionID == "" {
		return nil
	}
	if err := m.store.RevokeSession(ctx, userID, sessionID); err != nil {
		return infrastructureError("revoke session", err)
	}
	return nil
}

func (m *Manager) RevokeUser(ctx context.Context, userID int64) (string, error) {
	sessionID, err := m.store.RevokeUser(ctx, userID)
	if err != nil {
		return "", infrastructureError("revoke user", err)
	}
	return sessionID, nil
}

func (m *Manager) issuePair(
	userID int64,
	session *Session,
	refreshToken string,
	now time.Time,
) (*TokenPair, error) {
	accessToken, err := m.issuer.IssueAccessToken(userID, session.SessionID, session.CredentialVersion)
	if err != nil {
		return nil, err
	}
	idleExpiresIn := session.IdleExpireAt - now.Unix()
	if idleExpiresIn < 0 {
		idleExpiresIn = 0
	}
	return &TokenPair{
		AccessToken:             accessToken,
		AccessExpiresIn:         m.config.AccessExpires,
		RefreshToken:            refreshToken,
		RefreshIdleExpiresIn:    idleExpiresIn,
		RefreshAbsoluteExpireAt: session.AbsoluteExpireAt,
		TokenType:               "Bearer",
	}, nil
}

func (m *Manager) mapRotateError(err error) error {
	switch {
	case errors.Is(err, ErrSessionReplaced):
		return sessionReplacedError()
	case errors.Is(err, ErrIdleExpired):
		return lifecycleError(401, AuthReasonRefreshIdleExpired, "登录会话已因长时间未使用而过期，请重新登录")
	case errors.Is(err, ErrAbsoluteExpired):
		return lifecycleError(401, AuthReasonSessionAbsoluteExpired, "登录会话已达到最长有效期，请重新登录")
	case errors.Is(err, ErrSessionNotFound),
		errors.Is(err, ErrDeviceMismatch),
		errors.Is(err, ErrRefreshInvalid):
		return refreshInvalidError(err)
	case errors.Is(err, ErrRefreshReused):
		return refreshReusedError(err)
	default:
		return infrastructureError("rotate refresh token", err)
	}
}

func refreshInvalidError(cause error) error {
	return &AuthError{
		Code:    401,
		Reason:  AuthReasonRefreshTokenInvalid,
		Message: "刷新凭证无效，请重新登录",
		Cause:   cause,
	}
}

func refreshReusedError(cause error) error {
	return &AuthError{
		Code:    409,
		Reason:  AuthReasonRefreshTokenReused,
		Message: "刷新请求已失效，请重试",
		Cause:   cause,
	}
}

func sessionReplacedError() error {
	return &AuthError{
		Code:    401,
		Reason:  AuthReasonSessionReplaced,
		Message: "账号已在其他设备登录",
		Cause:   ErrSessionReplaced,
	}
}

func (m *Manager) Session(ctx context.Context, sessionID string) (*Session, error) {
	session, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, infrastructureError("get session", err)
	}
	if session == nil {
		return nil, refreshInvalidError(ErrSessionNotFound)
	}
	return session, nil
}
