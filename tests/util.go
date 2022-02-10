package tests

import (
	"bytes"
	"crypto/rand"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"

	echo "github.com/labstack/echo/v4"
)

func NewMultipartFileRequest(method, path string, reader io.Reader) (*http.Request, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "filename")
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, reader)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	return req, nil
}

func NewEchoMultipartFileContext(method, path string, reader io.Reader) (echo.Context, *httptest.ResponseRecorder, error) {
	req, err := NewMultipartFileRequest(method, path, reader)
	if err != nil {
		return nil, nil, err
	}
	c, rec := NewEchoContext(req)
	return c, rec, nil
}

func NewEchoContext(req *http.Request) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func GenerateRandomReader(size int) io.Reader {
	randomBytes := make([]byte, size)
	rand.Read(randomBytes)
	return bytes.NewReader(randomBytes)
}
