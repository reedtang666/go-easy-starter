package auth

const (
	AuthReasonAccessTokenInvalid     = "ACCESS_TOKEN_INVALID"
	AuthReasonAccessTokenExpired     = "ACCESS_TOKEN_EXPIRED"
	AuthReasonSessionAbsoluteExpired = "SESSION_ABSOLUTE_EXPIRED"
	AuthReasonRefreshIdleExpired     = "REFRESH_IDLE_EXPIRED"
	AuthReasonRefreshTokenInvalid    = "REFRESH_TOKEN_INVALID"
	AuthReasonRefreshTokenReused     = "REFRESH_TOKEN_REUSED"
	AuthReasonSessionReplaced        = "SESSION_REPLACED"
)
