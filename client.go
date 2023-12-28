package elevate

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/smithy-go"
)

func NewManagementAPIClient(ctx context.Context) (*apigatewaymanagementapi.Client, error) {
	callbackURL := callbackURLFromContext(ctx)
	proxyCtx := ProxyRequestContext(ctx)
	if proxyCtx.APIID == "" {
		// for local with dummy credentials
		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-east-1"
		}
		if callbackURL == "" {
			return nil, errors.New("elevate: callbackURL is empty")
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
			o.BaseEndpoint = &callbackURL
		}), nil
	}
	awsConfig := awsConfigFromContext(ctx)
	if callbackURL == "" {
		if strings.HasPrefix(proxyCtx.DomainName, proxyCtx.APIID) &&
			strings.HasSuffix(proxyCtx.DomainName, "amazonaws.com") &&
			proxyCtx.Stage != "" {
			callbackURL = "https://" + proxyCtx.DomainName + "/" + proxyCtx.Stage
		} else {
			callbackURL = "https://" + proxyCtx.APIID + ".execute-api." + awsConfig.Region + ".amazonaws.com/" + proxyCtx.Stage
		}
	}
	return apigatewaymanagementapi.NewFromConfig(awsConfig, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(callbackURL)
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
	var apiErr smithy.APIError
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
