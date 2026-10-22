package contexts

import "context"

type Identity struct {
	UserID            int64
	SessionID         string
	CredentialVersion int64
}
type identityKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}
func Get(ctx context.Context) Identity { id, _ := ctx.Value(identityKey{}).(Identity); return id }
