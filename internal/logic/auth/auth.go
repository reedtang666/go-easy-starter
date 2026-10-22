package auth

import (
	"context"
	"errors"
	core "github.com/reedtang666/go-easy-starter/internal/library/auth"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

type sAuth struct {
	users  service.IUser
	tokens *core.Manager
}

func New(users service.IUser, tokens *core.Manager) *sAuth {
	return &sAuth{users: users, tokens: tokens}
}
func (s *sAuth) Login(ctx context.Context, username, password string, device core.Device) (*core.TokenPair, error) {
	u, e := s.users.Authenticate(ctx, username, password)
	if e != nil {
		return nil, e
	}
	if device.ID == "" {
		return nil, response.Fail(400, "DEVICE_ID_REQUIRED", "deviceId不能为空")
	}
	p, _, e := s.tokens.Login(ctx, u.Id, u.CredentialVersion, device)
	return p, MapError(e)
}
func (s *sAuth) Refresh(ctx context.Context, value, device string) (*core.TokenPair, error) {
	sid, e := core.ParseRefreshTokenSessionID(value)
	if e != nil {
		return nil, MapError(e)
	}
	session, e := s.tokens.Session(ctx, sid)
	if e != nil {
		return nil, MapError(e)
	}
	if e = s.checkUser(ctx, session.UserID, session.CredentialVersion); e != nil {
		return nil, e
	}
	p, e := s.tokens.Refresh(ctx, value, device)
	return p, MapError(e)
}
func (s *sAuth) Validate(ctx context.Context, value string) (contexts.Identity, error) {
	session, _, e := s.tokens.ValidateAccessWithClaims(ctx, value)
	if e != nil {
		return contexts.Identity{}, MapError(e)
	}
	if e = s.checkUser(ctx, session.UserID, session.CredentialVersion); e != nil {
		return contexts.Identity{}, e
	}
	return contexts.Identity{UserID: session.UserID, SessionID: session.SessionID, CredentialVersion: session.CredentialVersion}, nil
}
func (s *sAuth) ValidateSession(ctx context.Context, id contexts.Identity) error {
	_, e := s.tokens.ValidateSession(ctx, id.UserID, id.SessionID)
	if e != nil {
		return MapError(e)
	}
	return s.checkUser(ctx, id.UserID, id.CredentialVersion)
}
func (s *sAuth) Logout(ctx context.Context, id contexts.Identity) error {
	return MapError(s.tokens.Logout(ctx, id.UserID, id.SessionID))
}
func (s *sAuth) checkUser(ctx context.Context, id, version int64) error {
	u, e := s.users.Get(ctx, id)
	if e != nil {
		return e
	}
	if u.Status != 1 || u.CredentialVersion != version {
		return response.Fail(401, "SESSION_REVOKED", "登录凭证已失效")
	}
	return nil
}
func MapError(e error) error {
	if e == nil {
		return nil
	}
	var a *core.AuthError
	if errors.As(e, &a) {
		status := a.Code
		if status < 400 || status > 599 {
			status = 401
		}
		return response.Fail(status, a.Reason, a.Message)
	}
	if errors.Is(e, core.ErrMalformedRefreshToken) || errors.Is(e, core.ErrSessionNotFound) {
		return response.Fail(401, "REFRESH_TOKEN_INVALID", "刷新凭证无效")
	}
	return &response.Error{Status: 503, Reason: "AUTH_UNAVAILABLE", Message: "认证服务暂时不可用", Cause: e}
}
