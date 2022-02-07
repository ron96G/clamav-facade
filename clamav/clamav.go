package clamav

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/ron96G/go-common-utils/log"
)

var (
	CHUNK_SIZE     = 2048
	defaultMaxSize = int(25 * 1000 * 1000)
	DELIM          = []byte("\000\000\000\000")
)

// For Docs see https://manpages.debian.org/testing/clamav-daemon/clamd.8.en.html
type ClamavClient struct {
	Hostname        string
	Port            uint
	DefaultTimeout  time.Duration
	StreamMaxLength uint32
	Log             log.Logger
	MaxSize         int
	remoteAddr      *net.TCPAddr
	bufferPool      sync.Pool
}

func NewClamavClient(hostname string, port uint, timeout time.Duration) (c *ClamavClient, err error) {
	c = &ClamavClient{
		Hostname:       hostname,
		Port:           port,
		DefaultTimeout: timeout,
		MaxSize:        defaultMaxSize,
		Log:            log.New("clamav_client"),
		bufferPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(nil)
			},
		},
	}
	c.remoteAddr, err = net.ResolveTCPAddr("tcp", c.address())
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *ClamavClient) SetDefaultTimeout(timeout time.Duration) {
	c.DefaultTimeout = timeout
}

func (c *ClamavClient) SetMaxSize(size int) {
	c.MaxSize = size
}

func (c *ClamavClient) address() string {
	return fmt.Sprintf("%s:%d", c.Hostname, c.Port)
}

func (c *ClamavClient) getConn(ctx context.Context) (conn net.Conn, err error) {
	c.Log.Debug("connecting to clamav", "address", c.remoteAddr)
	conn, err = net.DialTCP("tcp", nil, c.remoteAddr)
	if err != nil {
		return nil, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.DefaultTimeout)
	}
	c.Log.Debug("setting deadline", "deadline", deadline)
	err = conn.SetDeadline(deadline)
	go func() {
		<-ctx.Done()
		c.Log.Debug("closing connection")
		c.releaseConn(conn)
	}()

	return
}

func (c *ClamavClient) releaseConn(conn net.Conn) {
	conn.Close()
}

func (c *ClamavClient) borrowBuffer() *bytes.Buffer {
	return c.bufferPool.Get().(*bytes.Buffer)
}

func (c *ClamavClient) releaseBuffer(buf *bytes.Buffer) {
	c.bufferPool.Put(buf)
}

func (c *ClamavClient) Ping(ctx context.Context) (ok bool) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return false
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte(`PING`))
	if err != nil {
		return false
	}

	buf := c.borrowBuffer()
	defer c.releaseBuffer(buf)
	_, err = io.Copy(buf, conn)
	if err != nil && err != io.EOF {
		return false
	}
	resp := buf.String()
	c.Log.Debug("successfully read ping response", "response", resp)
	return strings.TrimSpace(resp) == "PONG"
}

func (c *ClamavClient) Version(ctx context.Context) (version string, err error) {
	var conn net.Conn
	conn, err = c.getConn(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: failed to obtain connection", err)
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte(`VERSION`))
	if err != nil {
		return "", fmt.Errorf("%w: failed to write command", err)
	}

	buf := c.borrowBuffer()
	buf.Reset()
	defer c.releaseBuffer(buf)
	_, err = io.Copy(buf, conn)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("%w: failed to read response", err)
	}

	resp := string(bytes.Trim(buf.Bytes(), "\x00"))
	resp = strings.TrimSpace(resp)
	c.Log.Debug("Successfully read ping response", "response", resp)
	return resp, nil
}

func (c *ClamavClient) Reload(ctx context.Context) (err error) {
	var conn net.Conn
	conn, err = c.getConn(ctx)
	if err != nil {
		return fmt.Errorf("%w: failed to obtain connection", err)
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte(`RELOAD`))
	if err != nil {
		return fmt.Errorf("%w: failed to write command", err)
	}

	buf := c.borrowBuffer()
	buf.Reset()
	defer c.releaseBuffer(buf)
	_, err = io.Copy(buf, conn)
	if err != nil && err != io.EOF {
		return fmt.Errorf("%w: failed to read response", err)
	}
	c.Log.Debug("successfully read reload response", "response", buf.String())
	return nil
}

func (c *ClamavClient) Shutdown(ctx context.Context) {
	var conn net.Conn
	var err error
	conn, err = c.getConn(ctx)
	if err != nil {
		c.Log.Warn("failed to get conn", "error", err)
		return
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte(`SHUTDOWN`))
	if err != nil {
		c.Log.Warn("failed to write command to clamav", "command", "shutdown", "error", err)
		return
	}
}

func (c *ClamavClient) Stats(ctx context.Context) (stats string, err error) {
	var conn net.Conn
	conn, err = c.getConn(ctx)
	if err != nil {
		c.Log.Warn("failed to get conn", "error", err)
		return "", fmt.Errorf("%w: failed to obtain connection", err)
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte("zSTATS\000"))
	if err != nil {
		return "", fmt.Errorf("%w: failed to write command", err)
	}

	buf := c.borrowBuffer()
	buf.Reset()
	defer c.releaseBuffer(buf)
	_, err = io.Copy(buf, conn)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("%w: failed to read response", err)
	}

	resp := strings.TrimSpace(buf.String())
	c.Log.Debug("successfully read reload response", "response", resp)
	return resp, nil
}

func (c *ClamavClient) ScanFile(ctx context.Context, rawURL string) (ok bool, err error) {
	var obj io.Reader
	var n int

	n, obj, err = download(rawURL)
	if err != nil {
		n, obj, err = readFile(rawURL)
		if err != nil {
			return false, err
		}
	}
	c.Log.Debug("Trying to scan file", "filename", rawURL, "length", n)
	if !c.CheckFilesize(n) {
		return false, fmt.Errorf("file exceeded size limit")
	}
	return c.Scan(ctx, obj)
}

func (c *ClamavClient) Scan(ctx context.Context, obj io.Reader) (ok bool, err error) {
	var conn net.Conn
	var written int

	conn, err = c.getConn(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: failed to obtain connection", err)
	}

	_, err = conn.Write([]byte("zINSTREAM\000"))
	if err != nil {
		return false, fmt.Errorf("%w: failed to write command", err)
	}

	chunk := make([]byte, CHUNK_SIZE)
	chunkSize := make([]byte, 4)
	for {

		n, err := obj.Read(chunk)
		if err != nil {
			if err != io.EOF {
				return false, fmt.Errorf("%w: failed to read chunk", err)
			}
			c.Log.Debug("Reached EOF", "sum_sent_bytes", written+n)
			break
		}

		binary.BigEndian.PutUint32(chunkSize, uint32(n))
		_, err = conn.Write(chunkSize)
		if err != nil {
			return false, fmt.Errorf("%w: failed to write chunksize", err)
		}

		writtenChunkSize, err := conn.Write(chunk[:n])
		if err != nil {
			return false, fmt.Errorf("%w: failed to write chunk", err)
		}
		written += n
		c.Log.Debug("written to clamav", "sent_bytes", written, "written_chunk", writtenChunkSize, "chunk_size", binary.BigEndian.Uint32(chunkSize))
	}

	_, err = conn.Write(DELIM)
	if err != nil {
		return false, fmt.Errorf("%w: failed to write termination", err)
	}

	c.Log.Info("successfully sent file to clamav", "sent_bytes", written)

	buf := c.borrowBuffer()
	buf.Reset()
	defer c.releaseBuffer(buf)

	_, err = io.Copy(buf, conn)
	if err != nil {
		if err != io.EOF {
			return false, fmt.Errorf("%w: failed to read response", err)
		}
		c.Log.Info("Buffer: ", "buffer", buf.String())
	}
	resp := buf.String()
	c.Log.Info("successfully read response", "response", resp)

	if !strings.Contains(resp, "OK") {
		// its a virus
		return false, nil
	}
	return true, nil
}

func (c *ClamavClient) CheckFilesize(n int) (ok bool) {
	c.Log.Debug("Checking file size", "size", n, "max", c.MaxSize)
	return !(n > c.MaxSize)
}
