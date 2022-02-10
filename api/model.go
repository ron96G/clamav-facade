package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	echo "github.com/labstack/echo/v4"
	log "github.com/ron96G/go-common-utils/log"
)

type Client interface {
	Scan(context.Context, io.Reader) (bool, error)
	ScanFile(context.Context, string) (bool, error)
	Stats(ctx context.Context) (string, error)
	Reload(ctx context.Context) error
	Version(ctx context.Context) (string, error)
	Ping(ctx context.Context) error
	Shutdown(ctx context.Context)
	CheckFilesize(int) bool
}

type API struct {
	Addr         string
	Prefix       string
	Log          log.Logger
	client       Client
	router       *echo.Echo
	server       *http.Server
	tlsCfg       *tls.Config
	StopChan     <-chan struct{}
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func (a *API) ToString() string {
	return fmt.Sprintf(
		"addr='%s', prefix='%s', read_timeout='%s', write_timeout='%s', idle_timeout='%s'",
		a.Addr, a.Prefix, a.ReadTimeout, a.WriteTimeout, a.IdleTimeout,
	)
}

type Result struct {
	ID      string      `json:"id,omitempty"`
	Status  string      `json:"status,omitempty"`
	Details interface{} `json:"details,omitempty"`
}
type Response struct {
	Results []Result `json:"results,omitempty"`
}

func newResponse() *Response {
	return &Response{
		Results: []Result{},
	}
}
