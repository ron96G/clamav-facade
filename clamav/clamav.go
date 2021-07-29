package clamav

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/ron96G/clamav-facade/util"
)

var (
	pong           = []byte("PONG")
	CHUNK_SIZE     = 2048
	defaultMaxSize = 25 * 1000 * 1000
)

// For Docs see https://manpages.debian.org/testing/clamav-daemon/clamd.8.en.html
type ClamavClient struct {
	Hostname        string
	Port            uint
	Timeout         time.Duration
	StreamMaxLength uint32
	Log             util.Logger
	MaxSize         int
	remoteAddr      *net.TCPAddr
}

func NewClamavClient(hostname string, port uint, timeout time.Duration) (c *ClamavClient, err error) {
	c = &ClamavClient{
		Hostname: hostname,
		Port:     port,
		Timeout:  timeout,
		MaxSize:  defaultMaxSize,
	}
	c.remoteAddr, err = net.ResolveTCPAddr("tcp", c.address())
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *ClamavClient) address() string {
	return fmt.Sprintf("%s:%d", c.Hostname, c.Port)
}

func (c *ClamavClient) getConn() (conn net.Conn, err error) {
	conn, err = net.DialTCP("tcp", nil, c.remoteAddr)
	if err != nil {
		return nil, err
	}
	err = conn.SetDeadline(time.Now().Add(c.Timeout))
	return
}

func (c *ClamavClient) releaseConn(conn net.Conn) {
	conn.Close()
}

func (c *ClamavClient) Ping() (ok bool) {
	var conn net.Conn
	var err error
	conn, err = c.getConn()
	if err != nil {
		return false
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte(`PING`))
	if err != nil {
		return false
	}

	buf := make([]byte, 4)

	_, err = conn.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	c.Log.Debugf("Successfully read ping response: %s", string(buf))
	return bytes.Equal(buf, pong)
}

func (c *ClamavClient) Version() (version string, err error) {
	var conn net.Conn
	conn, err = c.getConn()
	if err != nil {
		return "", fmt.Errorf("%w: failed to obtain connection", err)
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte(`VERSION`))
	if err != nil {
		return "", fmt.Errorf("%w: failed to write command", err)
	}

	buf := make([]byte, 1024)

	_, err = conn.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("%w: failed to read response", err)
	}

	resp := string(bytes.Trim(buf, "\x00"))
	resp = strings.TrimSpace(resp)
	c.Log.Debugf("Successfully read ping response: %s", resp)
	return resp, nil
}

func (c *ClamavClient) Reload() (err error) {
	var conn net.Conn
	conn, err = c.getConn()
	if err != nil {
		return fmt.Errorf("%w: failed to obtain connection", err)
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte(`RELOAD`))
	if err != nil {
		return fmt.Errorf("%w: failed to write command", err)
	}

	buf := make([]byte, 9)

	_, err = conn.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("%w: failed to read response", err)
	}
	c.Log.Debugf("Successfully read reload response: %s", string(buf))
	return nil
}

func (c *ClamavClient) Shutdown() {
	var conn net.Conn
	var err error
	conn, err = c.getConn()
	if err != nil {
		c.Log.Warn(err)
		return
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte(`SHUTDOWN`))
	if err != nil {
		c.Log.Warn(err)
		return
	}
}

func (c *ClamavClient) Stats() (stats string, err error) {
	var conn net.Conn
	conn, err = c.getConn()
	if err != nil {
		c.Log.Warn(err)
		return "", fmt.Errorf("%w: failed to obtain connection", err)
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte("zSTATS\000"))
	if err != nil {
		c.Log.Warn(err)
		return "", fmt.Errorf("%w: failed to write command", err)
	}

	buf := make([]byte, 1024)

	_, err = conn.Read(buf)
	if err != nil && err != io.EOF {
		c.Log.Warn(err)
		return "", fmt.Errorf("%w: failed to read response", err)
	}
	resp := string(bytes.Trim(buf, "\x00"))
	resp = strings.TrimSpace(resp)
	c.Log.Debugf("Successfully read stats response: %s", resp)
	return resp, nil
}

func (c *ClamavClient) ScanFile(rawURL string) (ok bool, err error) {
	var obj io.Reader
	var n int

	n, obj, err = download(rawURL)
	if err != nil {
		n, obj, err = readFile(rawURL)
		if err != nil {
			return false, err
		}
	}
	c.Log.Debugf("File '%s' has length %d", rawURL, n)
	if !c.CheckFilesize(n) {
		return false, fmt.Errorf("file exceeded size limit")
	}
	return c.Scan(obj)
}

func (c *ClamavClient) Scan(obj io.Reader) (ok bool, err error) {
	var conn net.Conn
	var written int

	conn, err = c.getConn()
	if err != nil {
		return false, fmt.Errorf("%w: failed to obtain connection", err)
	}
	defer c.releaseConn(conn)

	_, err = conn.Write([]byte("zINSTREAM\000"))
	if err != nil {
		return false, fmt.Errorf("%w: failed to write command", err)
	}

	for {
		chunk := make([]byte, CHUNK_SIZE)

		n, err := obj.Read(chunk)
		if err != nil {
			if err != io.EOF {
				return false, fmt.Errorf("%w: failed to read chunk", err)
			}
			c.Log.Debugf("Reached EOF after %d bytes", written+n)
			break
		}

		chunkSize := make([]byte, 4)
		binary.BigEndian.PutUint32(chunkSize, uint32(n))
		c.Log.Tracef("Writing to clamd\nWrite: %d\nWritten: %d. Chunksize: %v", n, written, chunkSize)

		_, err = conn.Write(chunkSize)
		if err != nil {
			return false, fmt.Errorf("%w: failed to write chunksize", err)
		}

		_, err = conn.Write(chunk)
		if err != nil {
			return false, fmt.Errorf("%w: failed to write chunk", err)
		}
		written += n
	}

	_, err = conn.Write([]byte("\000"))
	if err != nil {
		return false, fmt.Errorf("%w: failed to write termination", err)
	}
	buf := make([]byte, 1024)

	_, err = conn.Read(buf)
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("%w: failed to read response", err)
	}
	resp := string(buf)
	c.Log.Debugf("Successfully read stats response: %s", resp)

	if !strings.Contains(resp, "OK") {
		// its a virus
		return false, nil
	}
	return true, nil
}

func (c *ClamavClient) CheckFilesize(n int) (ok bool) {
	return !(n > c.MaxSize)
}
