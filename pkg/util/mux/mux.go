package mux

import (
	"net/http"
)

// MiddlewareFunc is the function signature of middlewares. The middleware is expected
// to return a handler that does work and then either writes a response, or calls the
// provided handler for additional processing.
type MiddlewareFunc func(http.Handler) http.Handler

// Handler wraps http.Handler and allows selectively running middleware on a matched route
type Handler interface {
	http.Handler
	Matched() bool
}

// muxHandler is a wrapper around http.Handler,
// used to differentiate between http.ServeMux internal
// handlers and handlers from this package.
type muxHandler struct {
	handler http.Handler
}

// ServeHTTP calls the wrapped function of the same name.
func (mh *muxHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	mh.handler.ServeHTTP(rw, req)
}

func (mh *muxHandler) Matched() bool {
	return true
}

// rootHandler runs the router's NotFoundHandler if one is set,
// or returns a fixed NotFound error. Middlewares are run
// if the NotFound handler was registered as a match for the root path.
type rootHandler struct {
	r         *Router
	rootMatch bool
}

func (rh *rootHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if rh.r.NotFoundHandler != nil {
		rh.r.NotFoundHandler.ServeHTTP(rw, req)
	} else {
		rw.WriteHeader(http.StatusNotFound)
	}
}

func (rh *rootHandler) Matched() bool {
	return rh.rootMatch && rh.r.NotFoundHandler != nil
}

// Router wraps http.ServeMux, adding functionality to call
// middlewares on matched requests.
type Router struct {
	NotFoundHandler http.Handler

	sm          *http.ServeMux
	rootHandler *rootHandler
	middlewares []MiddlewareFunc
}

// NewRouter creates a new Router
func NewRouter() *Router {
	r := &Router{sm: http.NewServeMux()}
	r.rootHandler = &rootHandler{r: r}
	r.sm.Handle("/", r.rootHandler)
	return r
}

// Use registers one or more middlewares. Middlewares are only run
// when a route is matched.
func (r *Router) Use(mwfs ...MiddlewareFunc) {
	r.middlewares = append(r.middlewares, mwfs...)
}

// SubRouter registers a route pattern, and returns a new router. Middlewares
// will only run if the subrouter pattern was matched.
// Note that the base pattern for the subrouter is NOT automatically
// prefixed to paths registered beneath it.
func (r *Router) SubRouter(pattern string) *Router {
	sr := NewRouter()
	r.Handle(pattern, sr)
	return sr
}

// Handle registers a route pattern to call a handler
func (r *Router) Handle(pattern string, handler http.Handler) {
	handler = &muxHandler{handler: handler}
	if pattern == "/" && r.NotFoundHandler == nil {
		r.rootHandler.rootMatch = true
		r.NotFoundHandler = handler
	} else {
		r.sm.Handle(pattern, handler)
	}
}

// HandleFunc registers a route pattern to call a handler function
func (r *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.Handle(pattern, http.HandlerFunc(handler))
}

// ServeHTTP handles the request, running middlewares if a registered pattern
// has been matched.
func (r *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	next := http.Handler(r.sm)
	// fix up path for CONNECT requests so that they do not get redirected
	if req.Method == http.MethodConnect && req.URL.Path == "" {
		req.URL.Path = "/"
	}
	// only run middlewares if this is a mux handler; other handlers are
	// http.ServeMux internal handlers that indicate no pattern was matched, and
	// we should not run middleware.
	handler, _ := r.sm.Handler(req)
	if h, ok := handler.(Handler); ok && h.Matched() {
		for i := len(r.middlewares) - 1; i >= 0; i-- {
			next = r.middlewares[i](next)
		}
	}
	next.ServeHTTP(rw, req)
}
