package elevate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

var (
	HTTPHeaderConnectionID = "Elevate-Connection-Id"
	HTTPHeaderRequestID    = "Elevate-Request-Id"
	HTTPHeaderEventType    = "Elevate-Event-Type"
	HTTPHeaderRouteKey     = "Elevate-Route-Key"
)

// NewRequest creates *net/http.Request from a Request.
func NewRequest(event json.RawMessage) (*http.Request, error) {
	return NewRequestWithContext(context.Background(), event)
}

// NewRequestWithContext creates *net/http.Request from a Request with context.
func NewRequestWithContext(ctx context.Context, event json.RawMessage) (*http.Request, error) {
	if ctx == nil {
		return nil, errors.New("github.com/mashiike/elevate.NewRequestWithContext: nil Context")
	}
	var proxyReq events.APIGatewayWebsocketProxyRequest
	dec := json.NewDecoder(bytes.NewReader(event))
	if err := dec.Decode(&proxyReq); err != nil {
		return nil, err
	}
	header := make(http.Header)
	for k, v := range proxyReq.MultiValueHeaders {
		for _, vv := range v {
			header.Add(k, vv)
		}
	}
	for k, v := range proxyReq.Headers {
		header.Set(k, v)
	}
	header.Del("Host")
	header.Set(HTTPHeaderConnectionID, proxyReq.RequestContext.ConnectionID)
	header.Set(HTTPHeaderEventType, proxyReq.RequestContext.EventType)
	header.Set(HTTPHeaderRouteKey, proxyReq.RequestContext.RouteKey)
	if header.Get(HTTPHeaderRequestID) == "" {
		header.Set(HTTPHeaderRequestID, proxyReq.RequestContext.RequestID)
	}

	u := url.URL{
		Scheme: "ws",
		Host:   proxyReq.RequestContext.DomainName,
		Path:   proxyReq.RequestContext.RouteKey,
	}
	var b io.Reader
	if proxyReq.IsBase64Encoded {
		raw := make([]byte, len(proxyReq.Body))
		n, err := base64.StdEncoding.Decode(raw, []byte(proxyReq.Body))
		if err != nil {
			return nil, err
		}
		b = bytes.NewReader(raw[0:n])
	} else {
		b = strings.NewReader(proxyReq.Body)
	}
	ctx = contextWithRequestContext(ctx, proxyReq.RequestContext)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), b)
	if err != nil {
		return nil, err
	}
	req.Header = header
	req.RemoteAddr = proxyReq.RequestContext.Identity.SourceIP
	return req, nil
}
