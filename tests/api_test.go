package tests

import (
	"bytes"
	"crypto/rand"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	echo "github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/ron96G/clamav-facade/api"
	"github.com/ron96G/clamav-facade/clamav"
	"github.com/ron96G/go-common-utils/log"
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}

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

var _ = Describe("API", func() {
	defer GinkgoRecover()

	randomFile := GenerateRandomReader(4096)

	log.Configure("debug", "json", os.Stdout)
	mock := NewMockServer("localhost", "33100")
	go mock.Run()

	client, _ := clamav.NewClamavClient("localhost", 33100, time.Second*10)
	client.SetMaxSize(4096)
	stopChan := make(chan struct{})
	api := api.NewAPI("", "localhost:32123", client, stopChan, log.New("api_logger"), nil)

	Describe("Scan Fails", func() {
		Describe("Due to wrong content-type", func() {
			c, rec := NewEchoContext(httptest.NewRequest(http.MethodPost, "/scan", randomFile))

			err := api.Scan(c)
			It("Should fail due to wrong content-type", func() {
				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusBadRequest))
				Expect(rec.Body.String()).To(ContainSubstring("\"status\":\"failed\""))
				Expect(rec.Body.String()).To(ContainSubstring("request Content-Type isn't multipart/form-data"))
			})
		})

		Describe("Due to wrong file-size", func() {
			slightlyLargerFile := GenerateRandomReader(4097)
			c, rec, err := NewEchoMultipartFileContext(http.MethodPost, "/scan", slightlyLargerFile)
			if err != nil {
				Fail(err.Error())
			}
			err = api.Scan(c)
			It("Should fail due to file size limit exceeded", func() {
				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusBadRequest))
				Expect(rec.Body.String()).To(ContainSubstring("\"status\":\"failed\""))
				Expect(rec.Body.String()).To(ContainSubstring("file size limit exceeded"))
			})
		})

		Describe("Due to clamav error", func() {
			mock.Expect("zINSTREAM", 1, "FAIL")
			c, rec, err := NewEchoMultipartFileContext(http.MethodPost, "/scan", randomFile)
			if err != nil {
				Fail(err.Error())
			}
			err = api.Scan(c)
			It("Should fail due a clamav error", func() {
				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusBadGateway))
				Expect(rec.Body.String()).To(ContainSubstring("\"status\":\"failed\""))
				Expect(rec.Body.String()).To(ContainSubstring("failed to read response"))
			})
		})
	})

	Describe("Scan Success", func() {
		Describe("With no virus", func() {
			mock.Expect("zINSTREAM", 1, "OK")
			c, rec, err := NewEchoMultipartFileContext(http.MethodPost, "/scan", randomFile)
			if err != nil {
				Fail(err.Error())
			}
			err = api.Scan(c)
			It("Should succeed with no virus", func() {
				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("\"status\":\"success\""))
				Expect(rec.Body.String()).To(ContainSubstring("file does not contains a virus"))
			})
		})

		Describe("With virus found", func() {
			mock.Expect("zINSTREAM", 1, "VIRUS")
			c, rec, err := NewEchoMultipartFileContext(http.MethodPost, "/scan", randomFile)
			if err != nil {
				Fail(err.Error())
			}
			err = api.Scan(c)
			It("Should succeed with virus", func() {
				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("\"status\":\"virus\""))
				Expect(rec.Body.String()).To(ContainSubstring("file contains a virus"))
			})
		})
	})
})
