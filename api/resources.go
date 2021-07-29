package api

import (
	"time"

	echo "github.com/labstack/echo/v4"
)

func (a *API) Scan(e echo.Context) error {
	req := e.Request()
	resp := newResponse()
	statusCode := 200

	req.ParseMultipartForm(32 << 20) // limit the maximum memory when parsing request to 32MB

	for key, headers := range req.MultipartForm.File {
		file, _, err := req.FormFile(key)
		if err != nil {
			resp.Results = append(resp.Results, Result{ID: key, Status: "failed", Details: err.Error()})
			return returnJSON(e, 400, resp)
		}
		defer file.Close()

		if !a.client.CheckFilesize(int(headers[0].Size)) {
			resp.Results = append(resp.Results, Result{ID: key, Status: "failed", Details: "file size limit exceeded"})
			statusCode = 400
			break
		}
		start := time.Now()
		ok, err := a.client.Scan(file)
		if err != nil {
			resp.Results = append(resp.Results, Result{ID: key, Status: "failed", Details: err.Error()})
			statusCode = 502
			break

		} else {
			a.Log.Infof("Scanned file '%s' with length %2.4fmb in %vms with result %v",
				key, float64(headers[0].Size)/1024/1024, time.Since(start).Milliseconds(), ok,
			)
			if !ok {
				resp.Results = append(resp.Results, Result{ID: key, Status: "virus", Details: "file contains a virus"})
				statusCode = 200

			} else {
				resp.Results = append(resp.Results, Result{ID: key, Status: "success", Details: "file does not contains a virus"})
			}
		}
	}

	return returnJSON(e, statusCode, resp)
}

func (a *API) Ping(e echo.Context) (err error) {
	ok := a.client.Ping()
	resp := newResponse()
	statusCode := 200

	if !ok {
		resp.Results = append(resp.Results, Result{Status: "failed", Details: "clamav is not ready"})
		statusCode = 502

	} else {
		resp.Results = append(resp.Results, Result{Status: "success", Details: "clamav is ready"})
	}
	return returnJSON(e, statusCode, resp)
}

func (a *API) Reload(e echo.Context) error {
	err := a.client.Reload()
	resp := newResponse()

	statusCode := 201
	if err != nil {
		resp.Results = append(resp.Results, Result{Status: "failed", Details: err.Error()})
		statusCode = 502

	} else {
		resp.Results = append(resp.Results, Result{Status: "success", Details: "triggered reload"})
	}

	return returnJSON(e, statusCode, resp)
}

func (a *API) Stats(e echo.Context) error {
	stats, err := a.client.Stats()
	resp := newResponse()
	statusCode := 200

	if err != nil {
		resp.Results = append(resp.Results, Result{Status: "failed", Details: err.Error()})
		statusCode = 502
	} else {
		resp.Results = append(resp.Results, Result{Status: "success", Details: stats})
	}

	return returnJSON(e, statusCode, resp)
}
