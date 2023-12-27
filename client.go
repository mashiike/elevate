package elevate

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
)

func NewManagementAPIClient(ctx context.Context) (*apigatewaymanagementapi.Client, error) {
	runOptions := runOptionsFromContext(ctx)
	proxyCtx := ProxyContext(ctx)
	if proxyCtx.APIID == "" {
		// for local with dummy credentials
		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-east-1"
		}
		cfg := aws.Config{
			Region: region,
			Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
				"AWS_ACCESS_KEY_ID",
				"AWS_SECRET_ACCESS_KEY",
				"AWS_SESSION_TOKEN",
			)),
		}
		return apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
			o.BaseEndpoint = &runOptions.callbackURL
		}), nil
	}
	if proxyCtx.DomainName == "" {
		return nil, errors.New("elevate: DomainName is empty")
	}
	if proxyCtx.Stage == "" {
		return nil, errors.New("elevate: Stage is empty")
	}
	if runOptions.awsConfig == nil {
		return nil, errors.New("elevate: AWS Config is nil")
	}
	return apigatewaymanagementapi.NewFromConfig(*runOptions.awsConfig, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String("https://" + proxyCtx.DomainName + "/" + proxyCtx.Stage)
	}), nil
}
