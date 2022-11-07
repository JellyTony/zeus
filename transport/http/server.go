package http

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/JellyTony/zeus/internal/endpoint"
	"github.com/JellyTony/zeus/internal/host"
	"github.com/JellyTony/zeus/internal/matcher"
	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
	_ http.Handler         = (*Server)(nil)
)

// ServerOption is an HTTP server option.
type ServerOption func(*Server)

// Network with server network.
func Network(network string) ServerOption {
	return func(s *Server) {
		s.network = network
	}
}

// Address with server address.
func Address(addr string) ServerOption {
	return func(s *Server) {
		s.address = addr
	}
}

// Timeout with server timeout.
func Timeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.timeout = timeout
	}
}

// Middleware with service middleware option.
func Middleware(m ...middleware.Middleware) ServerOption {
	return func(o *Server) {
		o.middleware.Use(m...)
	}
}

// Filter with HTTP middleware option.
func Filter(filters ...FilterFunc) ServerOption {
	return func(o *Server) {
		o.filters = filters
	}
}

// RequestVarsDecoder with request decoder.
func RequestVarsDecoder(dec DecodeRequestFunc) ServerOption {
	return func(o *Server) {
		o.decVars = dec
	}
}

// RequestQueryDecoder with request decoder.
func RequestQueryDecoder(dec DecodeRequestFunc) ServerOption {
	return func(o *Server) {
		o.decQuery = dec
	}
}

// RequestDecoder with request decoder.
func RequestDecoder(dec DecodeRequestFunc) ServerOption {
	return func(o *Server) {
		o.decBody = dec
	}
}

// ResponseEncoder with response encoder.
func ResponseEncoder(en EncodeResponseFunc) ServerOption {
	return func(o *Server) {
		o.enc = en
	}
}

// ErrorEncoder with error encoder.
func ErrorEncoder(en EncodeErrorFunc) ServerOption {
	return func(o *Server) {
		o.ene = en
	}
}

// TLSConfig with TLS config.
func TLSConfig(c *tls.Config) ServerOption {
	return func(o *Server) {
		o.tlsConf = c
	}
}

// StrictSlash is with mux's StrictSlash
// If true, when the path pattern is "/path/", accessing "/path" will
// redirect to the former and vice versa.
func StrictSlash(strictSlash bool) ServerOption {
	return func(o *Server) {
		o.strictSlash = strictSlash
	}
}

// Listener with server lis
func Listener(lis net.Listener) ServerOption {
	return func(s *Server) {
		s.lis = lis
	}
}

// Server is an HTTP server wrapper.
type Server struct {
	*http.Server
	lis         net.Listener
	tlsConf     *tls.Config
	endpoint    *url.URL
	err         error
	network     string
	address     string
	timeout     time.Duration
	filters     []FilterFunc
	middleware  matcher.Matcher
	decVars     DecodeRequestFunc
	decQuery    DecodeRequestFunc
	decBody     DecodeRequestFunc
	enc         EncodeResponseFunc
	ene         EncodeErrorFunc
	strictSlash bool
	engine      *gin.Engine
}

// NewServer creates an HTTP server by options.
func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		network:     "tcp",
		address:     ":0",
		timeout:     1 * time.Second,
		middleware:  matcher.New(),
		decVars:     DefaultRequestVars,
		decQuery:    DefaultRequestQuery,
		decBody:     DefaultRequestDecoder,
		enc:         DefaultResponseEncoder,
		ene:         DefaultErrorEncoder,
		strictSlash: true,
	}
	for _, o := range opts {
		o(srv)
	}
	gin.SetMode(gin.ReleaseMode)
	srv.engine = gin.New()
	srv.engine.RedirectTrailingSlash = srv.strictSlash
	srv.engine.Use(srv.filter())

	srv.Server = &http.Server{
		Handler:   FilterChain(srv.filters...)(srv.engine),
		TLSConfig: srv.tlsConf,
	}
	return srv
}

// Use uses a service middleware with selector.
// selector:
//   - '/*'
//   - '/helloworld.v1.Greeter/*'
//   - '/helloworld.v1.Greeter/SayHello'
func (s *Server) Use(selector string, m ...middleware.Middleware) {
	s.middleware.Add(selector, m...)
}

// WalkRoute walks the router and all its sub-routers, calling walkFn for each route in the tree.
func (s *Server) WalkRoute(fn WalkRouteFunc) error {
	for _, route := range s.engine.Routes() {
		if err := fn(RouteInfo{Method: route.Method, Path: route.Path}); err != nil {
			return err
		}
	}

	return nil
}

// Route registers an HTTP router.
func (s *Server) Route(prefix string, ms ...middleware.Middleware) *Router {
	return newRouter(prefix, s, ms...)
}

// Handle registers a new route with a matcher for the URL path.
func (s *Server) Handle(path string, h http.Handler) {
	s.engine.Any(path, gin.WrapH(h))
}

// HandlePrefix registers a new route with a matcher for the URL path prefix.
func (s *Server) HandlePrefix(prefix string, h http.Handler) {
	s.engine.Any(prefix, gin.WrapH(h))
}

// HandleFunc registers a new route with a matcher for the URL path.
func (s *Server) HandleFunc(path string, h http.HandlerFunc) {
	s.engine.Any(path, gin.WrapF(h))
}

// ServeHTTP should write reply headers and data to the ResponseWriter and then return.
func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	s.Handler.ServeHTTP(res, req)
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) filter() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)
		if s.timeout > 0 {
			ctx, cancel = context.WithTimeout(c.Request.Context(), s.timeout)
		} else {
			ctx, cancel = context.WithCancel(c.Request.Context())
		}
		defer cancel()

		tr := &Transport{
			operation:    c.FullPath(),
			pathTemplate: c.FullPath(),
			reqHeader:    headerCarrier(c.Request.Header),
			replyHeader:  headerCarrier(c.Writer.Header()),
			request:      c.Request,
		}
		if s.endpoint != nil {
			tr.endpoint = s.endpoint.String()
		}
		tr.request = c.Request.WithContext(transport.NewServerContext(ctx, tr))
		c.Request = tr.request
		c.Next()
	}
}

// Endpoint return a real address to registry endpoint.
// examples:
//
//	https://127.0.0.1:8000
//	Legacy: http://127.0.0.1:8000?isSecure=false
func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndEndpoint(); err != nil {
		return nil, err
	}
	return s.endpoint, nil
}

// Start start the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	if err := s.listenAndEndpoint(); err != nil {
		return err
	}
	s.BaseContext = func(net.Listener) context.Context {
		return ctx
	}
	log.Infof("[HTTP] server listening on: %s", s.lis.Addr().String())
	var err error
	if s.tlsConf != nil {
		err = s.ServeTLS(s.lis, "", "")
	} else {
		err = s.Serve(s.lis)
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Stop stop the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	log.Info("[HTTP] server stopping")
	return s.Shutdown(ctx)
}

func (s *Server) listenAndEndpoint() error {
	if s.lis == nil {
		lis, err := net.Listen(s.network, s.address)
		if err != nil {
			s.err = err
			return err
		}
		s.lis = lis
	}
	if s.endpoint == nil {
		addr, err := host.Extract(s.address, s.lis)
		if err != nil {
			s.err = err
			return err
		}
		s.endpoint = endpoint.NewEndpoint(endpoint.Scheme("http", s.tlsConf != nil), addr)
	}
	return s.err
}
