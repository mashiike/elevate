package elevate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// TextMimeTypes is a list of identified as text.
var TextMimeTypes = []string{"image/svg+xml", "application/json", "application/xml"}

// DefaultContentType is a default content-type when missing in response.
var DefaultContentType = "text/plain; charset=utf-8"

// ResponeWriter represents a response writer implements http.ResponseWriter.
type ResponseWriter struct {
	bytes.Buffer
	header     http.Header
	statusCode int
}

func NewResponseWriter() *ResponseWriter {
	return &ResponseWriter{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (w *ResponseWriter) Header() http.Header {
	return w.header
}

func (w *ResponseWriter) WriteHeader(code int) {
	w.statusCode = code
}

func (w *ResponseWriter) Response() *events.APIGatewayProxyResponse {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	body := w.String()
	isBase64Encoded := false
	resp := &events.APIGatewayProxyResponse{
		StatusCode:        w.statusCode,
		Headers:           make(map[string]string),
		MultiValueHeaders: make(map[string][]string),
	}
	if w.header.Get("Content-Type") == "" {
		resp.Headers["Content-Type"] = DefaultContentType
	}
	for k := range w.header {
		v := w.header.Values(k)
		if len(v) == 0 {
			continue
		}
		if len(v) == 1 {
			resp.Headers[k] = v[0]
		}
		resp.MultiValueHeaders[k] = v
	}
	if isBinary(w.header) {
		isBase64Encoded = true
	}
	if isBase64Encoded {
		body = base64.StdEncoding.EncodeToString(w.Bytes())
	}
	resp.Body = body
	resp.IsBase64Encoded = isBase64Encoded
	return resp
}
func isBinary(header http.Header) bool {
	ct := header.Get("Content-Type")
	if ct != "" && !isTextMime(ct) {
		return true
	}
	ce := header.Get("Content-Encoding")
	if ce != "" && ce == "gzip" {
		return true
	}
	return false
}

func isTextMime(kind string) bool {
	mt, _, err := mime.ParseMediaType(kind)
	if err != nil {
		return false
	}

	if strings.HasPrefix(mt, "text/") {
		return true
	}

	isText := false
	for _, tmt := range TextMimeTypes {
		if mt == tmt {
			isText = true
			break
		}
	}
	return isText
}

// RouteKeySelector is a function to select route key from request body. for local.
type RouteKeySelector func(body []byte) (string, error)

// DefaultRouteKeySelector is a default RouteKeySelector. it returns "action" key from request body.
func DefaultRouteKeySelector(body []byte) (string, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", err
	}
	if action, ok := req["action"].(string); ok {
		return action, nil
	}
	return "action", nil
}

type runOptions struct {
	address          string
	runCtx           context.Context
	lambdaOptions    []lambda.Option
	awsConfig        *aws.Config
	listener         net.Listener
	callbackURL      string
	logger           *slog.Logger
	routeKeySelector RouteKeySelector
	varbose          bool
}

func defaultRunOptions() runOptions {
	return runOptions{
		address:          ":8080",
		runCtx:           context.Background(),
		logger:           slog.Default(),
		routeKeySelector: DefaultRouteKeySelector,
	}
}

type Option func(*runOptions)

// WithContext sets context.Context to runOptions.
func WithContext(ctx context.Context) Option {
	return func(o *runOptions) {
		o.runCtx = ctx
	}
}

// WithLocalAdress sets local address to runOptions.
func WithLocalAdress(address string) Option {
	return func(o *runOptions) {
		o.address = address

	}
}

// WithRouteKeySelector sets RouteKeySelector to runOptions. only for local.
func WithRouteKeySelector(selector RouteKeySelector) Option {
	return func(o *runOptions) {
		if selector != nil {
			o.routeKeySelector = selector
		} else {
			o.routeKeySelector = DefaultRouteKeySelector
		}
	}
}

// WithLambdaOptions sets lambda.Options to runOptions. only for AWS Lambda Runtime.
func WithLambdaOptions(options ...lambda.Option) Option {
	return func(o *runOptions) {
		o.lambdaOptions = append(o.lambdaOptions, options...)
	}
}

// WithAWSConfig sets aws.Config to runOptions. only for AWS Lambda Runtime. default is loaded default config.
func WithAWSConfig(config *aws.Config) Option {
	return func(o *runOptions) {
		o.awsConfig = config
	}
}

// WithListener sets net.Listener to runOptions. only for local.
func WithListener(listener net.Listener) Option {
	return func(o *runOptions) {
		o.listener = listener
	}
}

// WithLogger sets slog.Logger to runOptions.
func WithLogger(logger *slog.Logger) Option {
	return func(o *runOptions) {
		o.logger = logger
	}
}

// WithVerbose sets verbose mode to runOptions.
func WithVerbose() Option {
	return func(o *runOptions) {
		o.varbose = true
	}
}

func Run(mux http.Handler) error {
	return RunWithOptions(mux)
}

func RunWithOptions(mux http.Handler, options ...Option) error {
	runOpts := defaultRunOptions()
	for _, opt := range options {
		opt(&runOpts)
	}
	if strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		// on AWS Lambda Runtime
		if runOpts.awsConfig == nil {
			cfg, err := config.LoadDefaultConfig(runOpts.runCtx)
			if err != nil {
				return err
			}
			runOpts.awsConfig = &cfg
		}
		handler := func(ctx context.Context, event json.RawMessage) (*events.APIGatewayProxyResponse, error) {
			if runOpts.varbose {
				runOpts.logger.DebugContext(ctx, "lambda invoked", "event", string(event))
			}
			ctx = contextWithAWSConfig(ctx, *runOpts.awsConfig)
			if runOpts.callbackURL != "" {
				ctx = contextWithCallbackURL(ctx, runOpts.callbackURL)
			}
			req, err := NewRequestWithContext(ctx, event)
			if err != nil {
				return nil, err
			}
			w := &ResponseWriter{
				header: make(http.Header),
			}
			mux.ServeHTTP(w, req)
			return w.Response(), nil
		}
		runOpts.lambdaOptions = append(runOpts.lambdaOptions, lambda.WithContext(runOpts.runCtx))
		lambda.StartWithOptions(handler, runOpts.lambdaOptions...)
		return nil
	}
	slog.InfoContext(runOpts.runCtx, "starting up with local httpd", "address", runOpts.address)
	listener := runOpts.listener
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", runOpts.address)
		if err != nil {
			return err
		}
	} else {
		runOpts.address = listener.Addr().String()
		runOpts.listener = nil
	}
	defer listener.Close()
	runOpts.callbackURL = fmt.Sprintf("http://%s", listener.Addr().String())
	bridge := NewWebsocketHTTPBridgeHandler(mux)
	bridge.SetLogger(runOpts.logger)
	bridge.SetVerbose(runOpts.varbose)
	bridge.SetCallbackURL(runOpts.callbackURL)
	srv := http.Server{
		Addr:    runOpts.address,
		Handler: bridge,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	srvCtx, cancel := context.WithCancel(runOpts.runCtx)
	defer cancel()
	go func() {
		defer wg.Done()
		<-srvCtx.Done()
		slog.InfoContext(runOpts.runCtx, "shutting down local httpd", "address", runOpts.address)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	if err := srv.Serve(listener); err != nil {
		if err != http.ErrServerClosed {
			return err
		}
	}
	wg.Wait()
	return nil
}
