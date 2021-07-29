package api

import (
	"crypto/tls"
	"io"
	"net/http"

	echo "github.com/labstack/echo/v4"
	"github.com/ron96G/clamav-facade/util"
)

type Client interface {
	Scan(io.Reader) (bool, error)
	ScanFile(string) (bool, error)
	Stats() (string, error)
	Reload() error
	Version() (string, error)
	Ping() bool
	Shutdown()
	CheckFilesize(int) bool
}

type API struct {
	Addr     string
	Prefix   string
	Log      util.Logger
	client   Client
	router   *echo.Echo
	server   *http.Server
	tlsCfg   *tls.Config
	StopChan <-chan struct{}
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
