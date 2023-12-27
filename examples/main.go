package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/mashiike/elevate"
)

func main() {
	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	))
	err := elevate.Run(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bs, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		slog.Info(
			"received request",
			slog.String("path", r.URL.Path),
			slog.String("routeKey", r.Header.Get(elevate.HTTPHeaderRouteKey)),
			slog.String("connectionID", r.Header.Get(elevate.HTTPHeaderConnectionID)),
			slog.String("body", string(bs)),
		)
		if r.Header.Get(elevate.HTTPHeaderEventType) == "MESSAGE" {
			client, err := elevate.NewManagementAPIClient(r.Context())
			if err != nil {
				w.WriteHeader(500)
				w.Write([]byte(err.Error()))
				return
			}
			if bytes.EqualFold(bs, []byte("exit")) {
				_, err = client.DeleteConnection(r.Context(), &apigatewaymanagementapi.DeleteConnectionInput{
					ConnectionId: aws.String(r.Header.Get(elevate.HTTPHeaderConnectionID)),
				})
				if err != nil {
					slog.Error("delete connection failed", "detail", err, "connectionID", r.Header.Get(elevate.HTTPHeaderConnectionID))
					w.WriteHeader(500)
					w.Write([]byte(err.Error()))
					return
				}
			}
			_, err = client.PostToConnection(r.Context(), &apigatewaymanagementapi.PostToConnectionInput{
				ConnectionId: aws.String(r.Header.Get(elevate.HTTPHeaderConnectionID)),
				Data:         []byte("Echo! " + string(bs)),
			})
			if err != nil {
				slog.Error("post failed", "detail", err, "connectionID", r.Header.Get(elevate.HTTPHeaderConnectionID))
				w.WriteHeader(500)
				w.Write([]byte(err.Error()))
				return
			}
		}
		w.WriteHeader(200)
		if r.Header.Get(elevate.HTTPHeaderRouteKey) == "hello" {
			w.Write([]byte("Hello, `" + r.Header.Get(elevate.HTTPHeaderConnectionID) + "`!"))
		}
	}))
	if err != nil {
		slog.Error("run failed", "detail", err)
		os.Exit(1)
	}
}
