// credits: https://github.com/doejon/Go-ServerSentEventProxy
package proxy

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/ananchev/homeconnect-proxy/internal/logger"
)

//Std http handler interface implementation
type SSEProxy interface {
	http.Handler
}

// Create a new sse proxy
func NewSSEProxy(baseURL string) (SSEProxy, error) {

	director := func(request *http.Request) {
		// Form the request
		fullURL := baseURL + request.URL.Path
		u, err := url.Parse(fullURL)
		if err != nil {
			logger.Error("Error parsing sse url: '{error}'", "error", err)
			return
		}
		request.URL = u
		request.Host = u.Host

		// Obtain access token
		token, err := getToken()
		if err != nil {
			err_descr := "Error getting access token: " + err.Error()
			logger.Error(err_descr)
			err = errors.New(err_descr)
			return
		}

		// Add all necessary headers
		request.Header.Add("Authorization", "Bearer "+token.AccessToken)
		request.Header.Add("X-Forwarded-Host", request.Host)
		request.Header.Add("X-Origin-Host", u.Host)
		request.Header.Add("Content-Type", "text/event-stream")
		request.Header.Add("Cache-Control", "no-cache")
		request.Header.Add("Connection", "keep-alive")
		request.Header.Add("Transfer-Encoding", "chunked")
		request.Header.Add("Access-Control-Allow-Origin", "*")
	}

	proxy := &httputil.ReverseProxy{Director: director}
	proxy.FlushInterval = 100 * time.Millisecond

	return proxy, nil
}

func StartServerSentEventProxy() {

	sse, err := NewSSEProxy(BaseURL)

	if err != nil {
		err_descr := "Error starting SSE proxy: " + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}

	http.Handle("/homeappliances/{.*}/events", sse)
	http.Handle("/homeappliances/events", sse)

}
