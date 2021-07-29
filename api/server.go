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

	"github.com/labstack/echo-contrib/jaegertracing"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	echo_mw "github.com/labstack/echo/v4/middleware"
	"github.com/ron96G/clamav-facade/util"
)

func NewAPI(prefix, addr string, client Client, stopChan <-chan struct{}, logger util.Logger, tlsCfg *tls.Config) *API {
	api := &API{
		Prefix:   prefix,
		Addr:     addr,
		client:   client,
		router:   echo.New(),
		tlsCfg:   tlsCfg,
		StopChan: stopChan,
	}
	api.Log = logger

	// general middleware
	api.router.Use(echo_mw.Recover())
	subrouter := api.router.Group(prefix)
	subrouter.Use(echo_mw.Logger())

	// tracing middleware
	c := jaegertracing.New(api.router, nil)
	go func() {
		<-api.StopChan
		c.Close()
	}()

	// metrics middleware
	p := prometheus.NewPrometheus("clamav_facade", func(c echo.Context) bool {
		return strings.HasPrefix(c.Path(), "/health") || strings.HasPrefix(c.Path(), "/ping") || c.Path() == "/"
	})
	p.Use(api.router)

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
		IdleTimeout:  30 * time.Second,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	var listener net.Listener
	listener, err := net.Listen("tcp", a.Addr)
	if err != nil {
		a.Log.Fatal(err)
	}

	if a.tlsCfg != nil {
		listener = tls.NewListener(listener, a.tlsCfg)
	}

	schema := "http"
	if a.tlsCfg != nil {
		schema = "https"
	}

	go func() {
		a.Log.Infof("Starting API server on address %s://%s%s", schema, a.Addr, a.Prefix)
		if err := a.server.Serve(listener); err != http.ErrServerClosed && err != nil {
			a.Log.Fatal(err)
		}
	}()

	//  handle shutdown
	<-a.StopChan

	a.Log.Warn("Shutting down API")
	if err := a.server.Shutdown(context.TODO()); err != nil {
		a.Log.Fatalf("api server shutdown failed: %v", err)
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
