package elevate

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
)

type contextKey string

var (
	reqContextKey       = contextKey("elevate.RequestContext")
	awsConfigContextKey = contextKey("elevate.awsConfig")
	runOptsContextKey   = contextKey("elevate.runOptions")
)

func contextWithRequestContext(ctx context.Context, reqCtx events.APIGatewayWebsocketProxyRequestContext) context.Context {
	return context.WithValue(ctx, reqContextKey, reqCtx)
}

func contextWithRunOptions(ctx context.Context, opts runOptions) context.Context {
	return context.WithValue(ctx, runOptsContextKey, opts)
}

func ProxyContext(ctx context.Context) events.APIGatewayWebsocketProxyRequestContext {
	if ctx == nil {
		return events.APIGatewayWebsocketProxyRequestContext{}
	}
	if v, ok := ctx.Value(reqContextKey).(events.APIGatewayWebsocketProxyRequestContext); ok {
		return v
	}
	return events.APIGatewayWebsocketProxyRequestContext{}
}

func runOptionsFromContext(ctx context.Context) runOptions {
	if ctx == nil {
		return defaultRunOptions()
	}
	if v, ok := ctx.Value(runOptsContextKey).(runOptions); ok {
		return v
	}
	return defaultRunOptions()
}
