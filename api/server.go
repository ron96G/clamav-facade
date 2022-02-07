package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	log "github.com/ron96G/go-common-utils/log"

	"github.com/labstack/echo-contrib/jaegertracing"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	echo_mw "github.com/labstack/echo/v4/middleware"
)

var (
	OpsSkipper = func(c echo.Context) bool {
		return strings.HasPrefix(c.Path(), "/health") || strings.HasPrefix(c.Path(), "/ping") || c.Path() == "/metrics"
	}

	LoggerConfig = echo_mw.LoggerConfig{
		Skipper: OpsSkipper,
		Format: `{"time":"${time_custom}","id":"${id}","remote_ip":"${remote_ip}",` +
			`"method":"${method}","path":"${path}","user_agent":"${user_agent}",` +
			`"status_code":${status},"error":"${error}","elapsed_time":${latency}` +
			`,"request_length":${bytes_in},"response_length":${bytes_out}}` + "\n",
		CustomTimeFormat: time.RFC3339,
	}

	RequestIDConfig = echo_mw.RequestIDConfig{
		Skipper: OpsSkipper,
	}
)

func NewAPI(prefix, addr string, client Client, stopChan <-chan struct{}, logger log.Logger, tlsCfg *tls.Config) *API {
	api := &API{
		Prefix:       prefix,
		Addr:         addr,
		client:       client,
		router:       echo.New(),
		tlsCfg:       tlsCfg,
		StopChan:     stopChan,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	api.Log = logger

	// general middleware
	api.router.Use(echo_mw.Recover())
	api.router.Use(echo_mw.RequestIDWithConfig(RequestIDConfig))
	api.router.Use(echo_mw.LoggerWithConfig(LoggerConfig))

	// tracing middleware
	c := jaegertracing.New(api.router, nil)
	go func() {
		<-api.StopChan
		c.Close()
	}()

	// metrics middleware
	p := prometheus.NewPrometheus("clamav_facade", OpsSkipper)
	p.Use(api.router)

	subrouter := api.router.Group(prefix)
	// resources
	subrouter.POST("/scan", api.Scan)
	subrouter.PUT("/reload", api.Reload)
	subrouter.GET("/stats", api.Stats)
	subrouter.GET("/health", api.Ping)
	subrouter.GET("/", api.Ping)

	return api
}

func (a *API) Run() {
	a.server = &http.Server{
		Addr:         a.Addr,
		Handler:      a.router,
		IdleTimeout:  a.IdleTimeout,
		ReadTimeout:  a.ReadTimeout,
		WriteTimeout: a.WriteTimeout,
	}
	var listener net.Listener
	listener, err := net.Listen("tcp", a.Addr)
	if err != nil {
		a.Log.Error("failed to open tcp port", "error", err, "addr", a.Addr)
	}

	if a.tlsCfg != nil {
		listener = tls.NewListener(listener, a.tlsCfg)
	}

	schema := "http"
	if a.tlsCfg != nil {
		schema = "https"
	}

	go func() {
		a.Log.Debug(a.ToString())
		a.Log.Info("Starting API server", "addr", fmt.Sprintf("%s://%s%s", schema, a.Addr, a.Prefix))
		if err := a.server.Serve(listener); err != http.ErrServerClosed && err != nil {
			a.Log.Error("fatal error when starting server", "error", err)
		}
	}()

	//  handle shutdown
	<-a.StopChan

	a.Log.Warn("Shutting down API")
	if err := a.server.Shutdown(context.Background()); err != nil {
		a.Log.Error("api server shutdown failed", "error", err)
	}
}

func returnJSON(e echo.Context, statusCode int, obj interface{}) (err error) {
	resp := e.Response()

	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("%w: an unexpected error occured", err)
	}
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(statusCode)
	_, err = resp.Write(b)
	return
}
