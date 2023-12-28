package elevate_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mashiike/elevate"
)

type mutexBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (mb *mutexBuffer) Write(p []byte) (n int, err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	return mb.buf.Write(p)
}

func (mw *mutexBuffer) String() string {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	return mw.buf.String()
}

func TestWebsocketHTTPBridgeHandler(t *testing.T) {
	var mu sync.Mutex
	connectionIDs := make(map[string]struct{})
	handler := elevate.NewWebsocketHTTPBridgeHandler(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			connectionID := elevate.ConnectionID(req)
			switch elevate.RouteKey(req) {
			case "$connect":
				mu.Lock()
				connectionIDs[connectionID] = struct{}{}
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			case "$disconnect":
				mu.Lock()
				delete(connectionIDs, connectionID)
				mu.Unlock()
			case "$default":
				bs, err := io.ReadAll(req.Body)
				if err != nil {
					slog.Error("read body failed", "detail", err)
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
					return
				}
				if bytes.EqualFold(bs, []byte("exit")) {
					if err := elevate.DeleteConnection(req.Context(), connectionID); err != nil {
						slog.Error("delete connection failed", "detail", err)
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
					}
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, http.StatusText(http.StatusOK))
					return
				}
				if bytes.Equal(bs, []byte("me")) {
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, connectionID)
					return
				}
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, http.StatusText(http.StatusOK))
			case "echo":
				bs, err := io.ReadAll(req.Body)
				if err != nil {
					slog.Error("read body failed", "detail", err)
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write(bs)
			case "whisper":
				var data map[string]interface{}
				err := json.NewDecoder(req.Body).Decode(&data)
				if err != nil {
					slog.Error("unmarshal failed", "detail", err)
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, http.StatusText(http.StatusBadRequest))
					return
				}
				connectionID, ok := data["connectionID"].(string)
				if !ok {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, http.StatusText(http.StatusBadRequest))
					return
				}
				msg, ok := data["message"].(string)
				if !ok {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, http.StatusText(http.StatusBadRequest))
					return
				}
				err = elevate.PostToConnection(req.Context(), connectionID, []byte(msg))
				if err != nil {
					slog.Error("post to failed", "detail", err, "connectionID", connectionID)
					if !elevate.ConnectionIsGone(err) {
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
					} else {
						w.WriteHeader(http.StatusGone)
						fmt.Fprint(w, http.StatusText(http.StatusGone))
					}
					return
				}
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, http.StatusText(http.StatusOK))
			case "lookup":
				var data map[string]interface{}
				err := json.NewDecoder(req.Body).Decode(&data)
				if err != nil {
					slog.Error("unmarshal failed", "detail", err)
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, http.StatusText(http.StatusBadRequest))
					return
				}
				connectionID, ok := data["connectionID"].(string)
				if !ok {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, http.StatusText(http.StatusBadRequest))
					return
				}
				info, err := elevate.GetConnection(req.Context(), connectionID)
				if err != nil {
					slog.Error("get connection failed", "detail", err, "connectionID", connectionID)
					if !elevate.ConnectionIsGone(err) {
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
					} else {
						w.WriteHeader(http.StatusGone)
						fmt.Fprint(w, http.StatusText(http.StatusGone))
					}
					return
				}
				bs, err := json.Marshal(info)
				if err != nil {
					slog.Error("marshal failed", "detail", err, "connectionID", connectionID)
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write(bs)
			default:
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, http.StatusText(http.StatusNotFound))
			}
		}),
	)
	var buf mutexBuffer
	logger := slog.New(slog.NewJSONHandler(
		&buf,
		&slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	))
	t.Cleanup(func() {
		t.Log(buf.String())
	})
	handler.SetLogger(logger)
	handler.SetVerbose(true)
	server := httptest.NewServer(handler)
	defer server.Close()
	handler.SetCallbackURL(server.URL)
	connectURL := "ws://" + server.Listener.Addr().String() + "/"

	t.Run("echo route", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c, _, err := websocket.DefaultDialer.DialContext(ctx, connectURL, nil)
		if err != nil {
			log.Fatal("dial:", err)
		}
		defer func() {
			msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Normal Closure")
			c.WriteMessage(websocket.CloseMessage, msg)
			c.Close()
		}()
		if err := c.WriteMessage(websocket.TextMessage, []byte(`{"action":"echo", "hoge":"fuga"}`)); err != nil {
			log.Fatal("write:", err)
		}
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Fatal("read:", err)
		}
		if strings.TrimSpace(string(message)) != `{"action":"echo", "hoge":"fuga"}` {
			t.Errorf("unexpected message: `%s`", message)
		}
	})

	t.Run("send exit", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c, _, err := websocket.DefaultDialer.DialContext(ctx, connectURL, nil)
		if err != nil {
			log.Fatal("dial:", err)
		}
		defer func() {
			msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Normal Closure")
			c.WriteMessage(websocket.CloseMessage, msg)
			c.Close()
		}()
		if err := c.WriteMessage(websocket.TextMessage, []byte(`exit`)); err != nil {
			log.Fatal("write:", err)
		}
		_, _, err = c.ReadMessage()
		if err == nil {
			t.Errorf("unexpected message: `%s`", err)
		}
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) {
			t.Errorf("unexpected error: `%s`", err)
		}
		if closeErr.Code != websocket.CloseNormalClosure {
			t.Errorf("unexpected error: `%s`", err)
		}
	})

	t.Run("whisper", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c1, _, err := websocket.DefaultDialer.DialContext(ctx, connectURL, nil)
		if err != nil {
			log.Fatal("dial:", err)
		}
		c2, _, err := websocket.DefaultDialer.DialContext(ctx, connectURL, nil)
		if err != nil {
			log.Fatal("dial:", err)
		}
		defer func() {
			msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Normal Closure")
			c1.WriteMessage(websocket.CloseMessage, msg)
			c2.WriteMessage(websocket.CloseMessage, msg)
			c1.Close()
			c2.Close()
		}()
		if err := c1.WriteMessage(websocket.TextMessage, []byte(`me`)); err != nil {
			log.Fatal("write:", err)
		}
		_, connectionIDOfC1, err := c1.ReadMessage()
		if err != nil {
			log.Fatal("read:", err)
		}
		if _, ok := connectionIDs[string(connectionIDOfC1)]; !ok {
			t.Errorf("unexpected connectionID: `%s`", connectionIDOfC1)
		}
		if err := c2.WriteMessage(websocket.TextMessage, []byte(`{"action":"whisper", "connectionID":"`+string(connectionIDOfC1)+`", "message":"hello"}`)); err != nil {
			log.Fatal("write:", err)
		}
		_, resp, err := c2.ReadMessage()
		if err != nil {
			log.Fatal("read:", err)
		}
		if string(resp) != http.StatusText(http.StatusOK) {
			t.Errorf("unexpected response: `%s`", resp)
		}
		_, message, err := c1.ReadMessage()
		if err != nil {
			log.Fatal("read:", err)
		}
		if string(message) != "hello" {
			t.Errorf("unexpected message: `%s`", message)
		}
	})

	t.Run("lookup", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c1, _, err := websocket.DefaultDialer.DialContext(ctx, connectURL, nil)
		if err != nil {
			log.Fatal("dial:", err)
		}
		c2, _, err := websocket.DefaultDialer.DialContext(ctx, connectURL, nil)
		if err != nil {
			log.Fatal("dial:", err)
		}
		defer func() {
			msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Normal Closure")
			c1.WriteMessage(websocket.CloseMessage, msg)
			c2.WriteMessage(websocket.CloseMessage, msg)
			c1.Close()
			c2.Close()
		}()
		if err := c1.WriteMessage(websocket.TextMessage, []byte(`me`)); err != nil {
			log.Fatal("write:", err)
		}
		_, connectionIDOfC1, err := c1.ReadMessage()
		if err != nil {
			log.Fatal("read:", err)
		}
		if _, ok := connectionIDs[string(connectionIDOfC1)]; !ok {
			t.Errorf("unexpected connectionID: `%s`", connectionIDOfC1)
		}
		if err := c2.WriteMessage(websocket.TextMessage, []byte(`{"action":"lookup", "connectionID":"`+string(connectionIDOfC1)+`"}`)); err != nil {
			log.Fatal("write:", err)
		}
		_, resp, err := c2.ReadMessage()
		if err != nil {
			log.Fatal("read:", err)
		}
		var info struct {
			Identity struct {
				SourceIP  string `json:"sourceIp"`
				UserAgent string `json:"userAgent"`
			} `json:"identity"`
		}
		if err := json.Unmarshal(resp, &info); err != nil {
			t.Errorf("unexpected response: `%s`", resp)
		}
		if !strings.HasPrefix(info.Identity.SourceIP, "127.0.0.1:") {
			t.Errorf("unexpected response: `%s`", info.Identity.SourceIP)
		}
		if !strings.EqualFold(info.Identity.UserAgent, "Go-http-client/1.1") {
			t.Errorf("unexpected response: `%s`", info.Identity.UserAgent)
		}
	})
}
