package elevate

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
)

type contextKey string

var (
	reqContextKey         = contextKey("elevate.RequestContext")
	awsConfigContextKey   = contextKey("elevate.awsConfig")
	callbackURLContextKey = contextKey("elevate.callbackURL")
)

func contextWithRequestContext(ctx context.Context, reqCtx events.APIGatewayWebsocketProxyRequestContext) context.Context {
	return context.WithValue(ctx, reqContextKey, reqCtx)
}

func contextWithAWSConfig(ctx context.Context, cfg aws.Config) context.Context {
	return context.WithValue(ctx, awsConfigContextKey, cfg)
}

func contextWithCallbackURL(ctx context.Context, url string) context.Context {
	return context.WithValue(ctx, callbackURLContextKey, url)
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

func awsConfigFromContext(ctx context.Context) aws.Config {
	if ctx == nil {
		return aws.Config{}
	}
	if v, ok := ctx.Value(awsConfigContextKey).(aws.Config); ok {
		return v
	}
	return aws.Config{}
}

func callbackURLFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(callbackURLContextKey).(string); ok {
		return v
	}
	return ""
}
