package web

import "context"

type ctxKey string

const sessionCtxKey ctxKey = "webSession"
const sessionIDCtxKey ctxKey = "webSessionID"

type sessionData struct {
	session   *Session
	sessionID string
}

func withSession(ctx context.Context, s *Session, sid string) context.Context {
	return context.WithValue(ctx, sessionCtxKey, &sessionData{session: s, sessionID: sid})
}

func getSessionFromCtx(ctx context.Context) (*Session, string, bool) {
	v := ctx.Value(sessionCtxKey)
	if v == nil {
		return nil, "", false
	}
	sd, ok := v.(*sessionData)
	if !ok || sd == nil || sd.session == nil {
		return nil, "", false
	}
	return sd.session, sd.sessionID, true
}
