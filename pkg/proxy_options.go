package pkg

import (
	"crypto/x509"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type Options struct {
	AccountBytes      int64
	BufferSize        int
	ZeroThreads       chan<- map[string]int64
	ReadDeadline      time.Duration
	DialTimeout       time.Duration
	HTTPServer        *http.Server
	Auth              Auth
	Sessions          Sessions
	Router            Router
	ConnectionTracker ConnectionTracker
	Accountant        Accountant
	Parser            UsernameParser
	Logger            *zap.Logger
	// Additional
	CACert *x509.Certificate
	CAKey  any
}

type Option func(*Options)

func WithZeroThreadsChannel(ch chan<- map[string]int64) Option {
	return func(options *Options) {
		options.ZeroThreads = ch
	}
}

func WithAccountBytes(bytes int64) Option {
	return func(options *Options) {
		options.AccountBytes = bytes
	}
}

func WithHTTPServer(srv *http.Server) Option {
	return func(options *Options) {
		options.HTTPServer = srv
	}
}

func WithBufferSize(bytes int) Option {
	return func(options *Options) {
		options.BufferSize = bytes
	}
}

func WithReadDeadline(readDeadline time.Duration) Option {
	return func(options *Options) {
		options.ReadDeadline = readDeadline
	}
}

func WithDialTimeout(dialTimeout time.Duration) Option {
	return func(options *Options) {
		options.DialTimeout = dialTimeout
	}
}

func WithAuth(auth Auth) Option {
	return func(options *Options) {
		options.Auth = auth
	}
}

func WithSessions(sessions Sessions) Option {
	return func(options *Options) {
		options.Sessions = sessions
	}
}

func WithRouter(router Router) Option {
	return func(options *Options) {
		options.Router = router
	}
}

func WithTracker(tracker ConnectionTracker) Option {
	return func(options *Options) {
		options.ConnectionTracker = tracker
	}
}

func WithAccountant(accountant Accountant) Option {
	return func(options *Options) {
		options.Accountant = accountant
	}
}

func WithUsernameParser(parser UsernameParser) Option {
	return func(options *Options) {
		options.Parser = parser
	}
}

func WithLogger(logger *zap.Logger) Option {
	return func(options *Options) {
		options.Logger = logger
	}
}

// Additional
func WithCA(caCertFile, caKeyFile string) Option {
	return func(options *Options) {
		caCert, caKey, err := loadX509KeyPair(caCertFile, caKeyFile)
		if err == nil {
			options.CACert = caCert
			options.CAKey = caKey
		}
	}
}
