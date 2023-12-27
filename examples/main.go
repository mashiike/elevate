package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/mashiike/elevate"
)

var connections map[string]string = make(map[string]string)

func handler(w http.ResponseWriter, req *http.Request) {
	connectionID := elevate.ConnectionID(req)
	switch elevate.RouteKey(req) {
	case "$connect":
		broadcast(req.Context(), []byte("Join member: "+connectionID))
		connections[connectionID] = "no name:" + connectionID
		slog.Info("join member", "connectionID", connectionID, "remoteAddr", req.RemoteAddr, "current", len(connections))
	case "$disconnect":
		name, ok := connections[connectionID]
		if !ok {
			return
		}
		delete(connections, connectionID)
		broadcast(req.Context(), []byte("Leave member: "+name))
		slog.Info("leave member", "connectionID", connectionID, "remoteAddr", req.RemoteAddr, "current", len(connections))
	case "$default":
		bs, err := io.ReadAll(req.Body)
		if err != nil {
			slog.Error("read body failed", "detail", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		slog.Info("default", "connectionID", connectionID, "remoteAddr", req.RemoteAddr, "body", string(bs))
		if bytes.EqualFold(bs, []byte("exit")) {
			if err := elevate.DeleteConnection(req.Context(), connectionID); err != nil {
				slog.Error("delete connection failed", "detail", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		name := connections[connectionID]
		msg := fmt.Sprintf("[%s] %s", name, string(bs))
		broadcast(req.Context(), []byte(msg))
	case "hello":
		var data map[string]interface{}
		err := json.NewDecoder(req.Body).Decode(&data)
		if err != nil {
			slog.Error("unmarshal failed", "detail", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		name, ok := data["name"].(string)
		if !ok {
			return
		}
		slog.Info("hello", "connectionID", connectionID, "remoteAddr", req.RemoteAddr, "name", name)
		msg := fmt.Sprintf("[%s] Hello!, my name is %s", connections[connectionID], name)
		broadcast(req.Context(), []byte(msg))
		connections[connectionID] = name
	}
}

func broadcast(ctx context.Context, msg []byte) {
	for k := range connections {
		err := elevate.PostToConnection(ctx, k, msg)
		if err != nil {
			slog.Debug("post to failed", "detail", err, "connectionID", k)
			if !elevate.ConnectionIsGone(err) {
				slog.Error("post to failed", "detail", err, "connectionID", k)
			} else {
				slog.Warn("connection is gone", "connectionID", k)
				delete(connections, k)
			}
		}
	}
}

func main() {
	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	))
	err := elevate.RunWithOptions(
		http.HandlerFunc(handler),
		elevate.WithLocalAdress(":8080"),
	)
	if err != nil {
		slog.Error("run failed", "detail", err)
		os.Exit(1)
	}
}
