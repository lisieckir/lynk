package web

import "context"

type ctxKey string

const (
	sessionCtxKey ctxKey = "webSession"
	userCtxKey    ctxKey = "webUser"
)

type WebUser struct {
	ID       int64
	Username string
	Role     string
}

func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, s)
}

func sessionFromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionCtxKey).(*Session)
	return s, ok
}

func withWebUser(ctx context.Context, u *WebUser) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

func webUserFromContext(ctx context.Context) (*WebUser, bool) {
	u, ok := ctx.Value(userCtxKey).(*WebUser)
	return u, ok
}
