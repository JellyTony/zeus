package http

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"mime/multipart"
	"net/url"

	"net/http"
	"time"

	bind "github.com/gin-gonic/gin/binding"
	"github.com/gin-gonic/gin/render"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http/binding"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
)

var _ Context = (*wrapper)(nil)

// Context is an HTTP Context.
type Context interface {
	context.Context
	Vars() url.Values
	Query() url.Values
	Form() url.Values
	Header() http.Header
	Request() *http.Request
	Response() http.ResponseWriter
	Middleware(middleware.Handler) middleware.Handler
	Bind(interface{}) error
	BindVars(interface{}) error
	BindQuery(interface{}) error
	BindForm(interface{}) error
	Returns(interface{}, error) error
	Result(int, interface{}) error
	JSON(int, interface{}) error
	XML(int, interface{}) error
	String(int, string) error
	Blob(int, string, []byte) error
	Stream(int, string, io.Reader) error
	Reset(http.ResponseWriter, *http.Request)

	HandlerName() string
	HandlerNames() []string
	Handler() gin.HandlerFunc
	FullPath() string
	Next()
	IsAborted() bool
	Abort()
	AbortWithStatus(code int)
	AbortWithStatusJSON(code int, jsonObj any)
	AbortWithError(code int, err error) *gin.Error
	Error(err error) *gin.Error
	Set(key string, value any)
	Get(key string) (value any, exists bool)
	MustGet(key string) any
	GetString(key string) (s string)
	GetBool(key string) (b bool)
	GetInt(key string) (i int)
	GetInt64(key string) (i64 int64)
	GetUint(key string) (ui uint)
	GetUint64(key string) (ui64 uint64)
	GetFloat64(key string) (f64 float64)
	GetTime(key string) (t time.Time)
	GetDuration(key string) (d time.Duration)
	GetStringSlice(key string) (ss []string)
	GetStringMap(key string) (sm map[string]any)
	GetStringMapString(key string) (sms map[string]string)
	GetStringMapStringSlice(key string) (smss map[string][]string)
	Param(key string) string
	AddParam(key, value string)
	DefaultQuery(key, defaultValue string) string
	GetQuery(key string) (string, bool)
	QueryArray(key string) (values []string)
	GetQueryArray(key string) (values []string, ok bool)
	QueryMap(key string) (dicts map[string]string)
	GetQueryMap(key string) (map[string]string, bool)
	PostForm(key string) (value string)
	DefaultPostForm(key, defaultValue string) string
	GetPostForm(key string) (string, bool)
	PostFormArray(key string) (values []string)
	GetPostFormArray(key string) (values []string, ok bool)
	PostFormMap(key string) (dicts map[string]string)
	GetPostFormMap(key string) (map[string]string, bool)
	FormFile(name string) (*multipart.FileHeader, error)
	MultipartForm() (*multipart.Form, error)
	SaveUploadedFile(file *multipart.FileHeader, dst string) error
	BindJSON(obj any) error
	BindXML(obj any) error
	BindYAML(obj any) error
	BindTOML(obj interface{}) error
	BindHeader(obj any) error
	BindUri(obj any) error
	MustBindWith(obj any, b bind.Binding) error
	ShouldBind(obj any) error
	ShouldBindJSON(obj any) error
	ShouldBindXML(obj any) error
	ShouldBindQuery(obj any) error
	ShouldBindYAML(obj any) error
	ShouldBindTOML(obj interface{}) error
	ShouldBindHeader(obj any) error
	ShouldBindUri(obj any) error
	ShouldBindWith(obj any, b bind.Binding) error
	ShouldBindBodyWith(obj any, bb bind.BindingBody) (err error)
	ClientIP() string
	RemoteIP() string
	ContentType() string
	IsWebsocket() bool
	Status(code int)
	GetHeader(key string) string
	GetRawData() ([]byte, error)
	SetSameSite(samesite http.SameSite)
	SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool)
	Cookie(name string) (string, error)
	Render(code int, r render.Render)
	HTML(code int, name string, obj any)
	IndentedJSON(code int, obj any)
	SecureJSON(code int, obj any)
	JSONP(code int, obj any)

	AsciiJSON(code int, obj any)
	PureJSON(code int, obj any)

	YAML(code int, obj any)
	TOML(code int, obj any)
	ProtoBuf(code int, obj any)

	Redirect(code int, location string)
	Data(code int, contentType string, data []byte)
	DataFromReader(code int, contentLength int64, contentType string, reader io.Reader, extraHeaders map[string]string)
	File(filepath string)
	FileFromFS(filepath string, fs http.FileSystem)
	FileAttachment(filepath, filename string)
	SSEvent(name string, message any)

	Negotiate(code int, config gin.Negotiate)
	NegotiateFormat(offered ...string) string
	SetAccepted(formats ...string)
}

type responseWriter struct {
	code int
	w    http.ResponseWriter
}

func (w *responseWriter) reset(res http.ResponseWriter) {
	w.w = res
	w.code = http.StatusOK
}
func (w *responseWriter) Header() http.Header        { return w.w.Header() }
func (w *responseWriter) WriteHeader(statusCode int) { w.code = statusCode }
func (w *responseWriter) Write(data []byte) (int, error) {
	w.w.WriteHeader(w.code)
	return w.w.Write(data)
}

type wrapper struct {
	*gin.Context
	router *Router
	req    *http.Request
	res    http.ResponseWriter
	w      responseWriter
}

func (c *wrapper) Header() http.Header {
	return c.req.Header
}

func (c *wrapper) Vars() url.Values {
	raws := mux.Vars(c.req)
	vars := make(url.Values, len(raws))
	for k, v := range raws {
		vars[k] = []string{v}
	}
	return vars
}

func (c *wrapper) Form() url.Values {
	if err := c.req.ParseForm(); err != nil {
		return url.Values{}
	}
	return c.req.Form
}

func (c *wrapper) Query() url.Values {
	return c.req.URL.Query()
}
func (c *wrapper) Request() *http.Request        { return c.req }
func (c *wrapper) Response() http.ResponseWriter { return c.res }
func (c *wrapper) Middleware(h middleware.Handler) middleware.Handler {
	if tr, ok := transport.FromServerContext(c.req.Context()); ok {
		return middleware.Chain(c.router.srv.middleware.Match(tr.Operation())...)(h)
	}
	return middleware.Chain(c.router.srv.middleware.Match(c.req.URL.Path)...)(h)
}
func (c *wrapper) Bind(v interface{}) error      { return c.router.srv.decBody(c.req, v) }
func (c *wrapper) BindVars(v interface{}) error  { return c.router.srv.decVars(c.req, v) }
func (c *wrapper) BindQuery(v interface{}) error { return c.router.srv.decQuery(c.req, v) }
func (c *wrapper) BindForm(v interface{}) error  { return binding.BindForm(c.req, v) }
func (c *wrapper) Returns(v interface{}, err error) error {
	if err != nil {
		return err
	}
	return c.router.srv.enc(&c.w, c.req, v)
}

func (c *wrapper) Result(code int, v interface{}) error {
	c.w.WriteHeader(code)
	return c.router.srv.enc(&c.w, c.req, v)
}

func (c *wrapper) JSON(code int, v interface{}) error {
	c.res.Header().Set("Content-Type", "application/json")
	c.res.WriteHeader(code)
	return json.NewEncoder(c.res).Encode(v)
}

func (c *wrapper) XML(code int, v interface{}) error {
	c.res.Header().Set("Content-Type", "application/xml")
	c.res.WriteHeader(code)
	return xml.NewEncoder(c.res).Encode(v)
}

func (c *wrapper) String(code int, text string) error {
	c.res.Header().Set("Content-Type", "text/plain")
	c.res.WriteHeader(code)
	_, err := c.res.Write([]byte(text))
	if err != nil {
		return err
	}
	return nil
}

func (c *wrapper) Blob(code int, contentType string, data []byte) error {
	c.res.Header().Set("Content-Type", contentType)
	c.res.WriteHeader(code)
	_, err := c.res.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (c *wrapper) Stream(code int, contentType string, rd io.Reader) error {
	c.res.Header().Set("Content-Type", contentType)
	c.res.WriteHeader(code)
	_, err := io.Copy(c.res, rd)
	return err
}

func (c *wrapper) Reset(res http.ResponseWriter, req *http.Request) {
	c.w.reset(res)
	c.res = res
	c.req = req
}

func (c *wrapper) Deadline() (time.Time, bool) {
	if c.req == nil {
		return time.Time{}, false
	}
	return c.req.Context().Deadline()
}

func (c *wrapper) Done() <-chan struct{} {
	if c.req == nil {
		return nil
	}
	return c.req.Context().Done()
}

func (c *wrapper) Err() error {
	if c.req == nil {
		return context.Canceled
	}
	return c.req.Context().Err()
}

func (c *wrapper) Value(key interface{}) interface{} {
	if c.req == nil {
		return nil
	}
	return c.req.Context().Value(key)
}
