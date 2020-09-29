package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ziyasal/distroxy/internal/pkg/common"

	"github.com/gin-contrib/pprof"
	"github.com/ziyasal/distroxy/pkg/distrox"
)

const (
	defaultServerRWTimeout = 5 * time.Second
)

type Server struct {
	addr           string
	pprofEnabled   bool
	cache          *distrox.Cache
	metricsEnabled bool
	logger         common.Logger
	srv            *http.Server
	readTimeout    time.Duration
	writeTimeout   time.Duration
}

type serverOption func(*Server)

func NewServer(addr string, c *distrox.Cache, opts ...serverOption) *Server {
	s := &Server{addr: addr, cache: c, logger: common.NewDefaultLogger()}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func WithPprof(enabled bool) serverOption {
	return func(h *Server) {
		h.pprofEnabled = enabled
	}
}

func WithServerReadTimeout(t time.Duration) serverOption {
	return func(h *Server) {
		h.readTimeout = t
	}
}

func WithServerWriteTimeout(t time.Duration) serverOption {
	return func(h *Server) {
		h.writeTimeout = t
	}
}

func WithLogger(l common.Logger) serverOption {
	return func(h *Server) {
		h.logger = l
	}
}

func WithMode(mode string) serverOption {
	return func(h *Server) {
		if mode == gin.ReleaseMode {
			gin.SetMode(gin.ReleaseMode)
		} else {
			gin.SetMode(gin.DebugMode)
		}
	}
}

func (s *Server) Run() error {
	r := s.newRouter()

	if s.pprofEnabled {
		pprof.Register(r, "dev/pprof")
	}

	s.srv = &http.Server{
		Addr:         s.addr,
		Handler:      r,
		ReadTimeout:  defaultServerRWTimeout,
		WriteTimeout: defaultServerRWTimeout,
	}

	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Err("http server startup failed", err)
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.cache.Reset()
	if err != nil {
		s.logger.Err("Failed to reset cache", err)
	}

	err = s.cache.Close()
	if err != nil {
		s.logger.Err("Failed to close cache", err)
	}

	return s.srv.Shutdown(ctx)
}
