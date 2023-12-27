package elevate

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/smithy-go"
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

// PostToConnection posts data to connectionID.
func PostToConnection(ctx context.Context, connectionID string, data []byte) error {
	client, err := NewManagementAPIClient(ctx)
	if err != nil {
		return err
	}
	_, err = client.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         data,
	})
	return err
}

// DeleteConnection deletes connectionID.
func DeleteConnection(ctx context.Context, connectionID string) error {
	client, err := NewManagementAPIClient(ctx)
	if err != nil {
		return err
	}
	_, err = client.DeleteConnection(ctx, &apigatewaymanagementapi.DeleteConnectionInput{
		ConnectionId: aws.String(connectionID),
	})
	return err
}

// GetConnection gets connectionID.
func GetConnection(ctx context.Context, connectionID string) (*apigatewaymanagementapi.GetConnectionOutput, error) {
	client, err := NewManagementAPIClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetConnection(ctx, &apigatewaymanagementapi.GetConnectionInput{
		ConnectionId: aws.String(connectionID),
	})
}

// ConnectionIsGone returns true if err is GoneException.
func ConnectionIsGone(err error) bool {
	var apiErr *smithy.GenericAPIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.ErrorCode() == "GoneException"
}

// ExitsConnection returns true if connectionID exists.
func ExitsConnection(ctx context.Context, connectionID string) (bool, error) {
	_, err := GetConnection(ctx, connectionID)
	if err != nil {
		if ConnectionIsGone(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
