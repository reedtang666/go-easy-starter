package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/util/gconv"
)

const defaultRedisPrefix = "starter:auth:"

type RedisStore struct {
	client        *gredis.Redis
	prefix        string
	sessionPrefix string
}

func NewRedisStore(client *gredis.Redis, prefix string) (*RedisStore, error) {
	if client == nil {
		return nil, errors.New("auth redis client is required")
	}
	if prefix == "" {
		prefix = defaultRedisPrefix
	}
	if strings.ContainsAny(prefix, " \t\r\n*?[]") {
		return nil, errors.New("auth redis prefix contains unsafe characters")
	}
	return &RedisStore{
		client:        client,
		prefix:        prefix,
		sessionPrefix: prefix + "session:",
	}, nil
}

func (s *RedisStore) CreateOrReplace(
	ctx context.Context,
	session *Session,
	ttl time.Duration,
) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("marshal auth session: %w", err)
	}
	ttlSeconds, err := redisTTLSeconds(ttl)
	if err != nil {
		return "", err
	}
	result, err := s.client.Eval(
		ctx,
		createOrReplaceScript,
		2,
		[]string{s.sessionKey(session.SessionID), s.userKey(session.UserID)},
		[]interface{}{session.SessionID, string(payload), ttlSeconds, s.sessionPrefix},
	)
	if err != nil {
		return "", fmt.Errorf("create or replace auth session: %w", err)
	}
	return result.String(), nil
}

func (s *RedisStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	value, err := s.client.Get(ctx, s.sessionKey(sessionID))
	if err != nil {
		return nil, fmt.Errorf("get auth session: %w", err)
	}
	if value.IsEmpty() {
		return nil, nil
	}
	var session Session
	if err = json.Unmarshal(value.Bytes(), &session); err != nil {
		return nil, fmt.Errorf("decode auth session: %w", err)
	}
	return &session, nil
}

func (s *RedisStore) GetUserSessionID(ctx context.Context, userID int64) (string, error) {
	value, err := s.client.Get(ctx, s.userKey(userID))
	if err != nil {
		return "", fmt.Errorf("get auth user session: %w", err)
	}
	return value.String(), nil
}

func (s *RedisStore) Rotate(ctx context.Context, input RotateInput) (*RotateResult, error) {
	ttlSeconds, err := redisTTLSeconds(input.TTL)
	if err != nil {
		return nil, err
	}
	result, err := s.client.Eval(
		ctx,
		rotateScript,
		3,
		[]string{
			s.sessionKey(input.SessionID),
			s.userKey(input.UserID),
			s.replayKey(input.PresentedTokenHash),
		},
		[]interface{}{
			input.SessionID,
			input.PresentedTokenHash,
			input.NewRefreshTokenHash,
			input.DeviceID,
			input.NowUnix,
			input.IdleExpireAt,
			input.PreviousRefreshGraceUntil,
			ttlSeconds,
			input.AppVersion,
			input.ReplayPayload,
			input.ReplayTTLSeconds,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("rotate auth refresh token: %w", err)
	}
	values := result.Interfaces()
	if len(values) == 0 {
		return nil, errors.New("empty auth rotation result")
	}
	code := gconv.String(values[0])
	switch code {
	case "OK":
		if len(values) != 2 {
			return nil, errors.New("missing auth rotated session")
		}
		var session Session
		if err = json.Unmarshal([]byte(gconv.String(values[1])), &session); err != nil {
			return nil, fmt.Errorf("decode rotated auth session: %w", err)
		}
		return &RotateResult{Session: &session}, nil
	case "REPLAY":
		if len(values) != 2 {
			return nil, errors.New("missing auth replay payload")
		}
		return &RotateResult{ReplayPayload: gconv.String(values[1])}, nil
	case "NOT_FOUND":
		return nil, ErrSessionNotFound
	case "REPLACED":
		return nil, ErrSessionReplaced
	case "IDLE_EXPIRED":
		return nil, ErrIdleExpired
	case "ABSOLUTE_EXPIRED":
		return nil, ErrAbsoluteExpired
	case "REUSED":
		return nil, ErrRefreshReused
	case "DEVICE_MISMATCH":
		return nil, ErrDeviceMismatch
	case "INVALID":
		return nil, ErrRefreshInvalid
	default:
		return nil, fmt.Errorf("unknown auth rotation result: %s", code)
	}
}

func (s *RedisStore) RevokeSession(ctx context.Context, userID int64, sessionID string) error {
	_, err := s.client.Eval(
		ctx,
		revokeSessionScript,
		2,
		[]string{s.sessionKey(sessionID), s.userKey(userID)},
		[]interface{}{sessionID},
	)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	return nil
}

func (s *RedisStore) RevokeUser(ctx context.Context, userID int64) (string, error) {
	result, err := s.client.Eval(
		ctx,
		revokeUserScript,
		1,
		[]string{s.userKey(userID)},
		[]interface{}{s.sessionPrefix},
	)
	if err != nil {
		return "", fmt.Errorf("revoke auth user: %w", err)
	}
	return result.String(), nil
}

func (s *RedisStore) sessionKey(sessionID string) string {
	return s.sessionPrefix + sessionID
}

func (s *RedisStore) userKey(userID int64) string {
	return s.prefix + "user:" + strconv.FormatInt(userID, 10)
}

func (s *RedisStore) replayKey(presentedTokenHash string) string {
	return s.prefix + "replay:" + presentedTokenHash
}

func redisTTLSeconds(ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, errors.New("auth redis TTL must be positive")
	}
	seconds := int64(ttl / time.Second)
	if ttl%time.Second != 0 {
		seconds++
	}
	if seconds <= 0 {
		seconds = 1
	}
	return seconds, nil
}

const createOrReplaceScript = `
local oldSessionId = redis.call('GET', KEYS[2])
if oldSessionId and oldSessionId ~= ARGV[1] then
  redis.call('DEL', ARGV[4] .. oldSessionId)
end
redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[3])
redis.call('SET', KEYS[2], ARGV[1], 'EX', ARGV[3])
return oldSessionId or ''
`

const rotateScript = `
local raw = redis.call('GET', KEYS[1])
if not raw then
  return {'NOT_FOUND'}
end

local session = cjson.decode(raw)
local currentSessionId = redis.call('GET', KEYS[2])
if not currentSessionId or currentSessionId ~= ARGV[1] then
  return {'REPLACED'}
end
if session.status ~= 'active' then
  return {'REPLACED'}
end

local now = tonumber(ARGV[5])
if now >= tonumber(session.absoluteExpireAt) then
  return {'ABSOLUTE_EXPIRED'}
end
if now >= tonumber(session.idleExpireAt) then
  return {'IDLE_EXPIRED'}
end
if session.deviceId ~= ARGV[4] then
  return {'DEVICE_MISMATCH'}
end

local currentMatches = session.refreshTokenHash == ARGV[2]
local previousMatches = session.previousRefreshTokenHash and session.previousRefreshTokenHash == ARGV[2]
if previousMatches then
  local replay = redis.call('GET', KEYS[3])
  if replay then
    return {'REPLAY', replay}
  end
  redis.call('DEL', KEYS[1], KEYS[2])
  return {'REUSED'}
end
if not currentMatches and not previousMatches then
  return {'INVALID'}
end

session.previousRefreshTokenHash = session.refreshTokenHash
session.previousRefreshGraceUntil = tonumber(ARGV[7])
session.refreshTokenHash = ARGV[3]
session.rotationCounter = tonumber(session.rotationCounter or 0) + 1
session.lastRefreshAt = now
session.idleExpireAt = tonumber(ARGV[6])
if ARGV[9] ~= '' then
  session.appVersion = ARGV[9]
end

local updated = cjson.encode(session)
redis.call('SET', KEYS[1], updated, 'EX', ARGV[8])
redis.call('EXPIRE', KEYS[2], ARGV[8])
if tonumber(ARGV[11]) > 0 then
  redis.call('SET', KEYS[3], ARGV[10], 'EX', ARGV[11])
end
return {'OK', updated}
`

const revokeSessionScript = `
redis.call('DEL', KEYS[1])
if redis.call('GET', KEYS[2]) == ARGV[1] then
  redis.call('DEL', KEYS[2])
end
return 1
`

const revokeUserScript = `
local sessionId = redis.call('GET', KEYS[1])
redis.call('DEL', KEYS[1])
if sessionId then
  redis.call('DEL', ARGV[1] .. sessionId)
  return sessionId
end
return ''
`
