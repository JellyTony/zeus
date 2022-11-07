package http

import (
	"context"
	"net/http"
	"path"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/middleware"
)

// WalkRouteFunc is the type of the function called for each route visited by Walk.
type WalkRouteFunc func(RouteInfo) error

// RouteInfo is an HTTP route info.
type RouteInfo struct {
	Path   string
	Method string
}

// HandlerFunc defines a function to serve HTTP requests.
type HandlerFunc func(Context) error

// Router is an HTTP router.
type Router struct {
	prefix  string
	pool    sync.Pool
	srv     *Server
	filters []middleware.Middleware
}

func newRouter(prefix string, srv *Server, filters ...middleware.Middleware) *Router {
	r := &Router{
		prefix:  prefix,
		srv:     srv,
		filters: filters,
	}
	r.pool.New = func() interface{} {
		return &wrapper{router: r}
	}
	return r
}

// Group returns a new router group.
func (r *Router) Group(prefix string, filters ...middleware.Middleware) *Router {
	var newFilters []middleware.Middleware
	newFilters = append(newFilters, r.filters...)
	newFilters = append(newFilters, filters...)
	return newRouter(path.Join(r.prefix, prefix), r.srv, newFilters...)
}

// Handle registers a new route with a matcher for the URL path and method.
func (r *Router) Handle(method, relativePath string, h HandlerFunc, filters ...middleware.Middleware) {
	next := func(c *gin.Context) {
		ctx := r.pool.Get().(*wrapper)
		ctx.Context = c
		ctx.Reset(c.Writer, c.Request)

		ms := make([]middleware.Middleware, 0, len(r.filters)+len(filters))
		ms = append(ms, r.filters...)
		ms = append(ms, filters...)
		chain := middleware.Chain(ms...)
		nt := func(cc context.Context, req interface{}) (interface{}, error) {
			err := h(ctx)
			if err != nil {
				r.srv.ene(c.Writer, c.Request, err)
			}
			return c.Writer, err
		}
		nt = chain(nt)
		_, _ = nt(c.Request.Context(), c.Request)
		ctx.Reset(nil, nil)
		ctx.Context = nil
		r.pool.Put(ctx)
	}

	r.srv.engine.Handle(method, path.Join(r.prefix, relativePath), next)
}

// GET registers a new GET route for a path with matching handler in the router.
func (r *Router) GET(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodGet, path, h, m...)
}

// HEAD registers a new HEAD route for a path with matching handler in the router.
func (r *Router) HEAD(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodHead, path, h, m...)
}

// POST registers a new POST route for a path with matching handler in the router.
func (r *Router) POST(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodPost, path, h, m...)
}

// PUT registers a new PUT route for a path with matching handler in the router.
func (r *Router) PUT(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodPut, path, h, m...)
}

// PATCH registers a new PATCH route for a path with matching handler in the router.
func (r *Router) PATCH(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodPatch, path, h, m...)
}

// DELETE registers a new DELETE route for a path with matching handler in the router.
func (r *Router) DELETE(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodDelete, path, h, m...)
}

// CONNECT registers a new CONNECT route for a path with matching handler in the router.
func (r *Router) CONNECT(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodConnect, path, h, m...)
}

// OPTIONS registers a new OPTIONS route for a path with matching handler in the router.
func (r *Router) OPTIONS(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodOptions, path, h, m...)
}

// TRACE registers a new TRACE route for a path with matching handler in the router.
func (r *Router) TRACE(path string, h HandlerFunc, m ...middleware.Middleware) {
	r.Handle(http.MethodTrace, path, h, m...)
}
