package clamav

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

func download(rawURL string) (n int, obj io.Reader, err error) {
	var resp *http.Response
	var body []byte

	uri, err := url.Parse(rawURL)
	if err != nil {
		return 0, nil, err
	}

	resp, err = http.DefaultClient.Get(uri.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	return len(body), bytes.NewReader(body), nil
}

func readFile(rawURL string) (n int, obj io.Reader, err error) {
	body, err := ioutil.ReadFile(rawURL)
	if err != nil {
		return 0, nil, err
	}
	return len(body), bytes.NewReader(body), nil
}
