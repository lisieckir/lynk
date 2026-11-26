package web

import "context"

type ctxKey string

const sessionCtxKey ctxKey = "webSession"

func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, s)
}

func sessionFromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionCtxKey).(*Session)
	return s, ok
}
