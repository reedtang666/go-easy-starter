package auth

import (
	"errors"
	"fmt"
)

var ErrMalformedRefreshToken = errors.New("malformed refresh token")

// AuthError is safe to map to an API response without comparing localized text.
type AuthError struct {
	Code    int
	Reason  string
	Message string
	Cause   error
}

func (e *AuthError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Reason
}

func (e *AuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func HasReason(err error, reason string) bool {
	var authErr *AuthError
	return errors.As(err, &authErr) && authErr.Reason == reason
}

func invalidAccessToken(cause error) error {
	return &AuthError{
		Code:    401,
		Reason:  AuthReasonAccessTokenInvalid,
		Message: "登录凭证无效，请重新登录",
		Cause:   cause,
	}
}

func expiredAccessToken() error {
	return &AuthError{
		Code:    401,
		Reason:  AuthReasonAccessTokenExpired,
		Message: "登录凭证已过期",
	}
}

func lifecycleError(code int, reason, message string) error {
	return &AuthError{Code: code, Reason: reason, Message: message}
}

func infrastructureError(operation string, cause error) error {
	return fmt.Errorf("auth %s: %w", operation, cause)
}
