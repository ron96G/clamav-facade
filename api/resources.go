package api

import (
	"mime/multipart"
	"time"

	echo "github.com/labstack/echo/v4"
)

func (a *API) Scan(e echo.Context) error {
	req := e.Request()
	resp := newResponse()
	var err error
	statusCode := 200

	if req.MultipartForm == nil || len(req.MultipartForm.File) == 0 {
		return returnJSON(e, 400, map[string]interface{}{"message": "invalid request type"})
	}

	// limit the maximum memory when parsing request to 32MB
	if err = req.ParseMultipartForm(32 << 20); err != nil {
		return returnJSON(e, 500, map[string]interface{}{"message": "failed to parse multipartform"})
	}

	var file multipart.File
	var ok bool
	for key, headers := range req.MultipartForm.File {

		file, _, err = req.FormFile(key)
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
		ok, err = a.client.Scan(file)
		if err != nil {
			resp.Results = append(resp.Results, Result{ID: key, Status: "failed", Details: err.Error()})
			statusCode = 502
			break

		} else {
			a.Log.Info("Scanned file",
				"filename", key,
				"length", float64(headers[0].Size)/1024/1024,
				"elapsed_time", time.Since(start).Milliseconds(),
				"result", ok,
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
