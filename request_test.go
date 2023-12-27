package elevate_test

import (
	"io"
	"os"
	"testing"

	"github.com/mashiike/elevate"
)

func TestNewRequest__Connect(t *testing.T) {
	bs, err := os.ReadFile("testdata/connect.json")
	if err != nil {
		t.Fatal(err)
	}
	req, err := elevate.NewRequest(bs)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Errorf("req.Method = %s; want GET", req.Method)
	}
	if req.Host != "abcdefghij.execute-api.ap-northeast-1.amazonaws.com" {
		t.Errorf("req.Host = %s; want abcdefghij.execute-api.ap-northeast-1.amazonaws.com", req.Host)
	}
	if req.URL.Path != "/$connect" {
		t.Errorf("req.URL.Path = %s; want /$connect", req.URL.Path)
	}
	if req.URL.Scheme != "ws" {
		t.Errorf("req.URL.Scheme = %s; want ws", req.URL.Scheme)
	}
	if req.Header.Get("Host") != "" {
		t.Errorf("req.Header.Get(\"Host\") = %s; want empty", req.Header.Get("Host"))
	}
	if req.Header.Get("Sec-WebSocket-Key") != "xxxxxxxxxxxxxxxxxxxxx==" {
		t.Errorf("req.Header.Get(\"Sec-WebSocket-Key\") = %s; want xxxxxxxxxxxxxxxxxxxxx==", req.Header.Get("Sec-WebSocket-Key"))
	}
	if req.Header.Get("Sec-WebSocket-Version") != "13" {
		t.Errorf("req.Header.Get(\"Sec-WebSocket-Version\") = %s; want 13", req.Header.Get("Sec-WebSocket-Version"))
	}
	if req.Header.Get("Sec-WebSocket-Extensions") != "permessage-deflate; client_max_window_bits" {
		t.Errorf("req.Header.Get(\"Sec-WebSocket-Extensions\") = %s; want permessage-deflate; client_max_window_bits", req.Header.Get("Sec-WebSocket-Extensions"))
	}
	if req.Header.Get("X-Amzn-Trace-Id") != "Root=1-60b4d1a5-7c9a0b7b7b7b7b7b7b7b7b7b" {
		t.Errorf("req.Header.Get(\"X-Amzn-Trace-Id\") = %s; want Root=1-60b4d1a5-7c9a0b7b7b7b7b7b7b7b7b7b", req.Header.Get("X-Amzn-Trace-Id"))
	}
	if req.Header.Get("X-Forwarded-For") != "192.168.1.1" {
		t.Errorf("req.Header.Get(\"X-Forwarded-For\") = %s; want 192.168.1.1", req.Header.Get("X-Forwarded-For"))
	}
	if req.Header.Get("X-Forwarded-Port") != "443" {
		t.Errorf("req.Header.Get(\"X-Forwarded-Port\") = %s; want 443", req.Header.Get("X-Forwarded-Port"))
	}
	if req.Header.Get("X-Forwarded-Proto") != "https" {
		t.Errorf("req.Header.Get(\"X-Forwarded-Proto\") = %s; want https", req.Header.Get("X-Forwarded-Proto"))
	}
	if req.Header.Get(elevate.HTTPHeaderConnectionID) != "ZZZZZZZZZZZZZZZ=" {
		t.Errorf("req.Header.Get(\"%s\") = %s; want ZZZZZZZZZZZZZZZ=", elevate.HTTPHeaderConnectionID, req.Header.Get(elevate.HTTPHeaderConnectionID))
	}
	if req.Header.Get(elevate.HTTPHeaderRequestID) != "YYYYYYYYYYY=" {
		t.Errorf("req.Header.Get(\"%s\") = %s; want YYYYYYYYYYY=", elevate.HTTPHeaderRequestID, req.Header.Get(elevate.HTTPHeaderRequestID))
	}
	if req.RemoteAddr != "192.168.1.1" {
		t.Errorf("req.RemoteAddr = %s; want 192.168.1.1", req.RemoteAddr)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "" {
		t.Errorf("req.Body = %s; want empty", string(body))
	}
	proxyCtx := elevate.ProxyContext(req.Context())
	if proxyCtx.ConnectionID != "ZZZZZZZZZZZZZZZ=" {
		t.Errorf("proxyCtx.ConnectionID = %s; want ZZZZZZZZZZZZZZZ=", proxyCtx.ConnectionID)
	}
	if proxyCtx.EventType != "CONNECT" {
		t.Errorf("proxyCtx.EventType = %s; want CONNECT", proxyCtx.EventType)
	}
	if proxyCtx.DomainName != "abcdefghij.execute-api.ap-northeast-1.amazonaws.com" {
		t.Errorf("proxyCtx.DomainName = %s; want abcdefghij.execute-api.ap-northeast-1.amazonaws.com", proxyCtx.DomainName)
	}
	if proxyCtx.RouteKey != "$connect" {
		t.Errorf("proxyCtx.RouteKey = %s; want $connect", proxyCtx.RouteKey)
	}
	if proxyCtx.Stage != "develop" {
		t.Errorf("proxyCtx.Stage = %s; want develop", proxyCtx.Stage)
	}
}

func TestNewRequest__Disconnect(t *testing.T) {
	bs, err := os.ReadFile("testdata/disconnect.json")
	if err != nil {
		t.Fatal(err)
	}
	req, err := elevate.NewRequest(bs)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Errorf("req.Method = %s; want GET", req.Method)
	}
	if req.Host != "abcdefghij.execute-api.ap-northeast-1.amazonaws.com" {
		t.Errorf("req.Host = %s; want abcdefghij.execute-api.ap-northeast-1.amazonaws.com", req.Host)
	}
	if req.URL.Path != "/$disconnect" {
		t.Errorf("req.URL.Path = %s; want /$disconnect", req.URL.Path)
	}
	if req.URL.Scheme != "ws" {
		t.Errorf("req.URL.Scheme = %s; want ws", req.URL.Scheme)
	}
	if req.Header.Get("Host") != "" {
		t.Errorf("req.Header.Get(\"Host\") = %s; want empty", req.Header.Get("Host"))
	}
	if req.Header.Get(elevate.HTTPHeaderConnectionID) != "ZZZZZZZZZZZZZZZ=" {
		t.Errorf("req.Header.Get(\"%s\") = %s; want ZZZZZZZZZZZZZZZ=", elevate.HTTPHeaderConnectionID, req.Header.Get(elevate.HTTPHeaderConnectionID))
	}
	if req.Header.Get(elevate.HTTPHeaderRequestID) != "YYYYYYYYYYW=" {
		t.Errorf("req.Header.Get(\"%s\") = %s; want YYYYYYYYYYW=", elevate.HTTPHeaderRequestID, req.Header.Get(elevate.HTTPHeaderRequestID))
	}
	if req.RemoteAddr != "192.168.1.1" {
		t.Errorf("req.RemoteAddr = %s; want 192.168.1.1", req.RemoteAddr)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "" {
		t.Errorf("req.Body = %s; want empty", string(body))
	}
	proxyCtx := elevate.ProxyContext(req.Context())
	if proxyCtx.ConnectionID != "ZZZZZZZZZZZZZZZ=" {
		t.Errorf("proxyCtx.ConnectionID = %s; want ZZZZZZZZZZZZZZZ=", proxyCtx.ConnectionID)
	}
	if proxyCtx.EventType != "DISCONNECT" {
		t.Errorf("proxyCtx.EventType = %s; want DISCONNECT", proxyCtx.EventType)
	}
	if proxyCtx.DomainName != "abcdefghij.execute-api.ap-northeast-1.amazonaws.com" {
		t.Errorf("proxyCtx.DomainName = %s; want abcdefghij.execute-api.ap-northeast-1.amazonaws.com", proxyCtx.DomainName)
	}
	if proxyCtx.RouteKey != "$disconnect" {
		t.Errorf("proxyCtx.RouteKey = %s; want $disconnect", proxyCtx.RouteKey)
	}
	if proxyCtx.Stage != "develop" {
		t.Errorf("proxyCtx.Stage = %s; want develop", proxyCtx.Stage)
	}
}

func TestNewRequest__Default(t *testing.T) {
	bs, err := os.ReadFile("testdata/default.json")
	if err != nil {
		t.Fatal(err)
	}
	req, err := elevate.NewRequest(bs)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Errorf("req.Method = %s; want GET", req.Method)
	}
	if req.Host != "abcdefghij.execute-api.ap-northeast-1.amazonaws.com" {
		t.Errorf("req.Host = %s; want abcdefghij.execute-api.ap-northeast-1.amazonaws.com", req.Host)
	}
	if req.URL.Path != "/$default" {
		t.Errorf("req.URL.Path = %s; want /default", req.URL.Path)
	}
	if req.URL.Scheme != "ws" {
		t.Errorf("req.URL.Scheme = %s; want ws", req.URL.Scheme)
	}
	if req.Header.Get("Host") != "" {
		t.Errorf("req.Header.Get(\"Host\") = %s; want empty", req.Header.Get("Host"))
	}
	if req.Header.Get(elevate.HTTPHeaderConnectionID) != "ZZZZZZZZZZZZZZZ=" {
		t.Errorf("req.Header.Get(\"%s\") = %s; want ZZZZZZZZZZZZZZZ=", elevate.HTTPHeaderConnectionID, req.Header.Get(elevate.HTTPHeaderConnectionID))
	}
	if req.Header.Get(elevate.HTTPHeaderRequestID) != "YYYYYYYYYYZ=" {
		t.Errorf("req.Header.Get(\"%s\") = %s; want YYYYYYYYYYZ=", elevate.HTTPHeaderRequestID, req.Header.Get(elevate.HTTPHeaderRequestID))
	}
	if req.RemoteAddr != "192.168.1.1" {
		t.Errorf("req.RemoteAddr = %s; want 192.168.1.1", req.RemoteAddr)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"abc":"def"}` {
		t.Errorf(`req.Body = %s; want {"abc":"def"}`, string(body))
	}
	proxyCtx := elevate.ProxyContext(req.Context())
	if proxyCtx.ConnectionID != "ZZZZZZZZZZZZZZZ=" {
		t.Errorf("proxyCtx.ConnectionID = %s; want ZZZZZZZZZZZZZZZ=", proxyCtx.ConnectionID)
	}
	if proxyCtx.EventType != "MESSAGE" {
		t.Errorf("proxyCtx.EventType = %s; want MESSAGE", proxyCtx.EventType)
	}
	if proxyCtx.DomainName != "abcdefghij.execute-api.ap-northeast-1.amazonaws.com" {
		t.Errorf("proxyCtx.DomainName = %s; want abcdefghij.execute-api.ap-northeast-1.amazonaws.com", proxyCtx.DomainName)
	}
	if proxyCtx.RouteKey != "$default" {
		t.Errorf("proxyCtx.RouteKey = %s; want $default", proxyCtx.RouteKey)
	}
	if proxyCtx.Stage != "develop" {
		t.Errorf("proxyCtx.Stage = %s; want develop", proxyCtx.Stage)
	}
}

func TestNewRequest__RouteHello(t *testing.T) {
	bs, err := os.ReadFile("testdata/hello.json")
	if err != nil {
		t.Fatal(err)
	}
	req, err := elevate.NewRequest(bs)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Errorf("req.Method = %s; want GET", req.Method)
	}
	if req.Host != "abcdefghij.execute-api.ap-northeast-1.amazonaws.com" {
		t.Errorf("req.Host = %s; want abcdefghij.execute-api.ap-northeast-1.amazonaws.com", req.Host)
	}
	if req.URL.Path != "/hello" {
		t.Errorf("req.URL.Path = %s; want /hello", req.URL.Path)
	}
	if req.URL.Scheme != "ws" {
		t.Errorf("req.URL.Scheme = %s; want ws", req.URL.Scheme)
	}
	if req.Header.Get("Host") != "" {
		t.Errorf("req.Header.Get(\"Host\") = %s; want empty", req.Header.Get("Host"))
	}
	if req.Header.Get(elevate.HTTPHeaderConnectionID) != "ZZZZZZZZZZZZZZZ=" {
		t.Errorf("req.Header.Get(\"%s\") = %s; want ZZZZZZZZZZZZZZZ=", elevate.HTTPHeaderConnectionID, req.Header.Get(elevate.HTTPHeaderConnectionID))
	}
	if req.Header.Get(elevate.HTTPHeaderRequestID) != "YYYYYYYYYYA=" {
		t.Errorf("req.Header.Get(\"%s\") = %s; want YYYYYYYYYYA=", elevate.HTTPHeaderRequestID, req.Header.Get(elevate.HTTPHeaderRequestID))
	}
	if req.RemoteAddr != "192.168.1.1" {
		t.Errorf("req.RemoteAddr = %s; want 192.168.1.1", req.RemoteAddr)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"action":"hello","hoge":fuga"}` {
		t.Errorf(`req.Body = %s; want {"action":"hello","hoge":fuga"}`, string(body))
	}
	proxyCtx := elevate.ProxyContext(req.Context())
	if proxyCtx.ConnectionID != "ZZZZZZZZZZZZZZZ=" {
		t.Errorf("proxyCtx.ConnectionID = %s; want ZZZZZZZZZZZZZZZ=", proxyCtx.ConnectionID)
	}
	if proxyCtx.EventType != "MESSAGE" {
		t.Errorf("proxyCtx.EventType = %s; want MESSAGE", proxyCtx.EventType)
	}
	if proxyCtx.DomainName != "abcdefghij.execute-api.ap-northeast-1.amazonaws.com" {
		t.Errorf("proxyCtx.DomainName = %s; want abcdefghij.execute-api.ap-northeast-1.amazonaws.com", proxyCtx.DomainName)
	}
	if proxyCtx.RouteKey != "hello" {
		t.Errorf("proxyCtx.RouteKey = %s; want hello", proxyCtx.RouteKey)
	}
	if proxyCtx.Stage != "develop" {
		t.Errorf("proxyCtx.Stage = %s; want develop", proxyCtx.Stage)
	}
}
