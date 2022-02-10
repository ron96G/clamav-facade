package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/ron96G/clamav-facade/api"
	"github.com/ron96G/clamav-facade/clamav"
	"github.com/ron96G/go-common-utils/log"
)

func TestScanAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scan API Suite")
}

var _ = Describe("API", func() {
	defer GinkgoRecover()

	log.Configure("debug", "json", os.Stdout)
	mock := NewMockServer("localhost", 33100)
	go mock.Run()

	randomFile := GenerateRandomReader(4096)

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
			mock.Expect(INSTREAM, 1, RETURN_FAIL)
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
			mock.Expect(INSTREAM, 1, RETURN_OK)
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
			mock.Expect(INSTREAM, 1, RETURN_VIRUS)
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

	Describe("Ping Success", func() {
		Describe("Ready", func() {
			c, rec := NewEchoContext(httptest.NewRequest(http.MethodGet, "/", nil))
			mock.Expect(PING, 1, RETURN_OK)
			err := api.Ping(c)
			It("Should succeed", func() {
				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("\"status\":\"success\""))
				Expect(rec.Body.String()).To(ContainSubstring("clamav is ready"))
			})
		})

		Describe("Healthy", func() {
			c, rec := NewEchoContext(httptest.NewRequest(http.MethodGet, "/health", nil))
			mock.Expect(PING, 1, RETURN_OK)
			err := api.Ping(c)
			It("Should succeed", func() {
				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("\"status\":\"success\""))
				Expect(rec.Body.String()).To(ContainSubstring("clamav is ready"))
			})
		})
	})

	Describe("Ping Fails", func() {
		Describe("Not Ready", func() {
			c, rec := NewEchoContext(httptest.NewRequest(http.MethodGet, "/", nil))
			mock.Expect(PING, 1, RETURN_FAIL)
			err := api.Ping(c)
			It("Should succeed", func() {
				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusBadGateway))
				Expect(rec.Body.String()).To(ContainSubstring("\"status\":\"failed\""))
				Expect(rec.Body.String()).To(ContainSubstring("clamav is not ready yet"))
			})
		})
	})

	Describe("Stats Success", func() {
		c, rec := NewEchoContext(httptest.NewRequest(http.MethodGet, "/stats", nil))
		mock.Expect(STATS, 1, RETURN_OK)
		err := api.Stats(c)
		It("Should succeed", func() {
			Expect(err).To(BeNil())
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("Reload Success", func() {
		c, rec := NewEchoContext(httptest.NewRequest(http.MethodPut, "/reload", nil))
		mock.Expect(RELOAD, 1, RETURN_OK)
		err := api.Stats(c)
		It("Should succeed", func() {
			Expect(err).To(BeNil())
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})
})
