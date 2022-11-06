package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
)

// FilterChain return middlewares wrapper
func FilterChain(m ...middleware.Middleware) gin.HandlerFunc {
	chain := middleware.Chain(m...)
	return func(c *gin.Context) {
		next := func(ctx context.Context, req interface{}) (interface{}, error) {
			c.Next()
			var err error
			if c.Writer.Status() >= http.StatusBadRequest {
				err = errors.Errorf(c.Writer.Status(), errors.UnknownReason, errors.UnknownReason)
			}
			return c.Writer, err
		}
		next = chain(next)
		ctx := NewGinContext(c.Request.Context(), c)
		c.Request = c.Request.WithContext(ctx)
		if ginCtx, ok := FromGinContext(ctx); ok {
			SetOperation(ctx, ginCtx.FullPath())
		}
		next(c.Request.Context(), c.Request)
	}
}

type ginKey struct{}

// NewGinContext returns a new Context that carries gin.Context value.
func NewGinContext(ctx context.Context, c *gin.Context) context.Context {
	return context.WithValue(ctx, ginKey{}, c)
}

// FromGinContext returns the gin.Context value stored in ctx, if any.
func FromGinContext(ctx context.Context) (c *gin.Context, ok bool) {
	c, ok = ctx.Value(ginKey{}).(*gin.Context)
	return
}
