package web

import (
	"database/sql"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/lisieckir/lynk/internal/bookmark"
)

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}
}

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Handler struct {
	db           *sql.DB
	service      *bookmark.Service
	sessions     *SessionStore
	logger       *slog.Logger
	secure       bool // Secure cookie flag (true in production)
	tmpl         *template.Template
	static       http.Handler
	loginLimiter *loginRateLimiter
}

type Config struct {
	SecureCookie bool
	SessionTTL   time.Duration
}

func NewHandler(db *sql.DB, logger *slog.Logger, cfg Config) (*Handler, error) {
	svc, err := bookmark.NewService(db)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	ttl := cfg.SessionTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	tmpl, err := template.New("").Funcs(templateFuncs()).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		staticSub = nil
	}
	var staticHandler http.Handler
	if staticSub != nil {
		staticHandler = http.StripPrefix("/static/", http.FileServer(http.FS(staticSub)))
	} else {
		staticHandler = http.NotFoundHandler()
	}
	return &Handler{
		db:           db,
		service:      svc,
		sessions:     NewSessionStore(ttl, 10000),
		logger:       logger,
		secure:       cfg.SecureCookie,
		tmpl:         tmpl,
		static:       staticHandler,
		loginLimiter: newLoginRateLimiter(10, time.Minute),
	}, nil
}
