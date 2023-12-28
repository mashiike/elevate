package elevate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func generateID(size int) (string, error) {
	randomBytes := make([]byte, size)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(randomBytes), nil
}

func writeCloseFrame(ws *websocket.Conn, code int, reason string) error {
	return ws.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		time.Now().Add(10*time.Second),
	)
}

type WebsocketHTTPBridgeHandler struct {
	Handler                http.Handler
	callbackURL            string
	routeKeySelector       RouteKeySelector
	mu                     sync.RWMutex
	logger                 *slog.Logger
	connections            map[string]*websocket.Conn
	connectionReq          map[string]*http.Request
	connectionConnectedAt  map[string]time.Time
	connectionLastActiveAt map[string]time.Time
	router                 *http.ServeMux
	verbose                bool
	websocket.Upgrader
}

func NewWebsocketHTTPBridgeHandler(handler http.Handler) *WebsocketHTTPBridgeHandler {
	h := &WebsocketHTTPBridgeHandler{
		Handler:                handler,
		routeKeySelector:       DefaultRouteKeySelector,
		callbackURL:            "http://localhost",
		logger:                 slog.Default(),
		connections:            make(map[string]*websocket.Conn),
		connectionReq:          make(map[string]*http.Request),
		connectionConnectedAt:  make(map[string]time.Time),
		connectionLastActiveAt: make(map[string]time.Time),
		router:                 http.NewServeMux(),
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
	h.router.HandleFunc("/", h.serveWebsocket)
	h.router.HandleFunc("/@connections/", h.serveConnections)
	return h
}

func (h *WebsocketHTTPBridgeHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

func (h *WebsocketHTTPBridgeHandler) SetVerbose(verbose bool) {
	h.verbose = verbose
}

func (h *WebsocketHTTPBridgeHandler) SetCallbackURL(callbackURL string) {
	h.callbackURL = callbackURL
}

func (h *WebsocketHTTPBridgeHandler) SetRouteKeySelector(selector RouteKeySelector) {
	h.routeKeySelector = selector
}

func (h *WebsocketHTTPBridgeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.logger.Debug("receive request", "method", req.Method, "path", req.URL.Path)
	h.router.ServeHTTP(w, req)
}

func (h *WebsocketHTTPBridgeHandler) serveWebsocket(w http.ResponseWriter, req *http.Request) {
	h.logger.Debug("start serve websocket", "method", req.Method, "path", req.URL.Path)
	conn, err := h.Upgrade(w, req, nil)
	if err != nil {
		h.logger.ErrorContext(req.Context(), "failed to upgrade", "detail", err)
		return
	}
	defer conn.Close()
	connectionID, err := h.onConnect(req, conn)
	if err != nil {
		h.logger.ErrorContext(req.Context(), "failed to connect", "detail", err)
		return
	}
	defer h.onDisonnect(connectionID)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Minute)); err != nil {
			h.logger.ErrorContext(req.Context(), "failed to set read deadline", "detail", err)
			h.removeFromConnectionList(connectionID, websocket.CloseInternalServerErr, "Cannot Set Read Deadline")
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			var closeErr *websocket.CloseError
			if err == io.EOF {
				h.logger.Debug("receive EOF")
				h.removeFromConnectionList(connectionID, 0, "")
			}
			if errors.As(err, &closeErr) {
				h.removeFromConnectionList(connectionID, 0, "")
				h.logger.Debug("receive close frame", "code", closeErr.Code, "reason", closeErr.Text)
				if closeErr.Code == websocket.CloseNormalClosure {
					return
				}
				h.logger.Warn("receive close frame", "code", closeErr.Code, "reason", closeErr.Text, "connection_id", connectionID)
			}
			h.logger.ErrorContext(req.Context(), "failed to receive message", "detail", err)
			h.removeFromConnectionList(connectionID, websocket.CloseInternalServerErr, "Cannot Receive Message")
			return
		}
		if err := h.onReceiveMessage(connectionID, conn, msg); err != nil {
			h.logger.ErrorContext(req.Context(), "failed to receive message", "detail", err)
			return
		}
	}
}

func (h *WebsocketHTTPBridgeHandler) serveConnections(w http.ResponseWriter, req *http.Request) {
	cid := req.URL.Path[len("/@connections/"):]
	uuidObj, err := uuid.NewRandom()
	if err != nil {
		h.logger.Error("@connections failed to generate uuid", "detail", err)
		w.Header().Set("X-Amzn-ErrorType", "InternalServerError")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger := h.logger.With("request_id", uuidObj.String(), "target_connection_id", cid)
	if h.verbose {
		logger.Info("@connections", "method", req.Method, "path", req.URL.Path)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Amzn-RequestId", uuidObj.String())
	conn, ok := h.getConn(cid)
	if !ok {
		w.Header().Set("X-Amzn-ErrorType", "GoneException")
		w.WriteHeader(http.StatusGone)
		return
	}
	defer req.Body.Close()
	switch req.Method {
	case http.MethodPost:
		bs, err := io.ReadAll(req.Body)
		if err != nil {
			logger.Error("@connections failed to read body", "detail", err)
			w.Header().Set("X-Amzn-ErrorType", "InternalServerError")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var messageType int
		if isBinary(req.Header) {
			messageType = websocket.BinaryMessage
		} else {
			messageType = websocket.TextMessage
		}
		if err := conn.WriteMessage(messageType, bs); err != nil {
			logger.Error("@connections failed to send message", "detail", err)
			w.Header().Set("X-Amzn-ErrorType", "InternalServerError")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	case http.MethodDelete:
		h.removeFromConnectionList(cid, websocket.CloseNormalClosure, "Connection Closed Normally")
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodGet:
		connectedAt, lastActiveAt, originReq := h.getConnectionInfo(cid)
		if originReq == nil {
			w.Header().Set("X-Amzn-ErrorType", "GoneException")
			w.WriteHeader(http.StatusGone)
			return
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		err := enc.Encode(map[string]interface{}{
			"identity": map[string]interface{}{
				"sourceIp":  originReq.RemoteAddr,
				"userAgent": originReq.UserAgent(),
			},
			"connectedAt":  connectedAt.Format(time.RFC3339),
			"lastActiveAt": lastActiveAt.Format(time.RFC3339),
		})
		if err != nil {
			logger.Error("@connections failed to encode response", "detail", err)
			w.Header().Set("X-Amzn-ErrorType", "InternalServerError")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		io.Copy(w, &buf)
		return
	}
	w.Header().Set("X-Amzn-ErrorType", "ResourceNotFoundException")
	w.WriteHeader(http.StatusNotFound)
}

func (h *WebsocketHTTPBridgeHandler) newBridgeRequest(
	ctx context.Context,
	connectionID string,
	host string,
	eventType string,
	routeKey string,
	body io.Reader,
) (*http.Request, error) {
	requestID, err := generateID(11)
	if err != nil {
		return nil, err
	}
	u := url.URL{
		Scheme: "ws",
		Host:   host,
		Path:   routeKey,
	}
	if body == nil {
		body = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		u.String(),
		body,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HTTPHeaderConnectionID, connectionID)
	req.Header.Set(HTTPHeaderRequestID, requestID)
	req.Header.Set(HTTPHeaderEventType, eventType)
	req.Header.Set(HTTPHeaderRouteKey, routeKey)
	return req, nil
}

func (h *WebsocketHTTPBridgeHandler) addToConnectionList(connectionID string, connectedAt time.Time, req *http.Request, ws *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[connectionID] = ws
	h.connectionReq[connectionID] = req.Clone(context.TODO())
	h.connectionConnectedAt[connectionID] = connectedAt
	h.connectionLastActiveAt[connectionID] = connectedAt
}

func (h *WebsocketHTTPBridgeHandler) removeFromConnectionList(connectionID string, code int, reason string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	conn, ok := h.connections[connectionID]
	if !ok {
		return false
	}
	delete(h.connections, connectionID)
	if code > 0 {
		if err := writeCloseFrame(conn, code, reason); err != nil {
			h.logger.Error("failed to write close frame", "detail", err, "connection_id", connectionID)
		}
	}
	return true
}

func (h *WebsocketHTTPBridgeHandler) getConn(connectionID string) (*websocket.Conn, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.connections[connectionID]
	return conn, ok
}

func (h *WebsocketHTTPBridgeHandler) popConnectionInfo(connectionID string) (time.Time, time.Time, *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	connectedAt := h.connectionConnectedAt[connectionID]
	lastActiveAt := h.connectionLastActiveAt[connectionID]
	req, ok := h.connectionReq[connectionID]
	if ok {
		delete(h.connectionConnectedAt, connectionID)
		delete(h.connectionLastActiveAt, connectionID)
		delete(h.connectionReq, connectionID)
	}
	return connectedAt, lastActiveAt, req
}

func (h *WebsocketHTTPBridgeHandler) getConnectionInfo(connectionID string) (time.Time, time.Time, *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	connectedAt := h.connectionConnectedAt[connectionID]
	lastActiveAt := h.connectionLastActiveAt[connectionID]
	req := h.connectionReq[connectionID]
	return connectedAt, lastActiveAt, req
}

func (h *WebsocketHTTPBridgeHandler) markActiveAt(connectionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connectionLastActiveAt[connectionID] = time.Now()
}

func (h *WebsocketHTTPBridgeHandler) getConnections() map[string]*websocket.Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	connections := make(map[string]*websocket.Conn)
	for k, v := range h.connections {
		connections[k] = v
	}
	return connections
}

func (h *WebsocketHTTPBridgeHandler) currentConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

func (h *WebsocketHTTPBridgeHandler) onConnect(originReq *http.Request, ws *websocket.Conn) (string, error) {
	now := time.Now()
	connectionID, err := generateID(11)
	if err != nil {
		writeCloseFrame(ws, websocket.CloseInternalServerErr, "failed to generate connection id")
		return "", err
	}
	h.logger.Debug("generate connection id", "connection_id", connectionID, "remote_addr", ws.RemoteAddr().String())
	req, err := h.newBridgeRequest(
		originReq.Context(),
		connectionID,
		originReq.Host,
		"CONNECT",
		"$connect",
		bytes.NewReader([]byte{}),
	)
	if err != nil {
		writeCloseFrame(ws, websocket.CloseInternalServerErr, "failed to create bridge request")
		return "", err
	}
	requsetID, err := generateID(11)
	if err != nil {
		writeCloseFrame(ws, websocket.CloseInternalServerErr, "failed to generate request id")
		return "", err
	}
	req.Header.Set(HTTPHeaderConnectionID, connectionID)
	req.Header.Set(HTTPHeaderRequestID, requsetID)
	req.Header.Set(HTTPHeaderEventType, "CONNECT")
	req.Header.Set(HTTPHeaderRouteKey, "$connect")
	proxyCtx := events.APIGatewayWebsocketProxyRequestContext{
		ConnectionID:      connectionID,
		RequestID:         requsetID,
		ExtendedRequestID: requsetID,
		EventType:         "CONNECT",
		RouteKey:          "$connect",
		DomainName:        originReq.Host,
		Identity: events.APIGatewayRequestIdentity{
			SourceIP: ws.RemoteAddr().String(),
		},
		ConnectedAt:      int64(now.UnixNano() / int64(time.Millisecond)),
		RequestTime:      now.Format(time.Layout),
		RequestTimeEpoch: int64(now.UnixNano() / int64(time.Millisecond)),
		MessageDirection: "IN",
	}
	req = req.WithContext(contextWithRequestContext(req.Context(), proxyCtx))
	h.logger.Debug("prepare connect bridge request", "connection_id", connectionID, "remote_addr", ws.RemoteAddr().String())
	respWriter := NewResponseWriter()
	h.Handler.ServeHTTP(respWriter, req)
	if respWriter.statusCode != http.StatusOK {
		h.logger.Debug("failed bridge handler", "status", respWriter.statusCode)
		writeCloseFrame(ws, websocket.CloseInternalServerErr, "failed bridge handler")
		return "", err
	}
	h.addToConnectionList(connectionID, now, originReq, ws)
	h.logger.Debug("connected", "connection_id", connectionID)
	if h.verbose {
		h.logger.Info("connected",
			"connection_id", connectionID,
			"remote_addr", ws.RemoteAddr().String(),
			"host", originReq.Host,
			"current_connections", h.currentConnections(),
		)
	}
	return connectionID, nil
}

func (h *WebsocketHTTPBridgeHandler) onDisonnect(connectionID string) {
	h.removeFromConnectionList(connectionID, websocket.CloseNormalClosure, "Connection Closed Normally")
	requsetID, err := generateID(11)
	if err != nil {
		requsetID = "00000000="
	}
	connectedAt, _, originReq := h.popConnectionInfo(connectionID)
	if originReq == nil {
		h.logger.Warn("connection info not found", "connection_id", connectionID)
		originReq = &http.Request{
			RemoteAddr: "unknown",
			Host:       "unknown",
		}
	}
	proxyCtx := events.APIGatewayWebsocketProxyRequestContext{
		ConnectionID:      connectionID,
		RequestID:         requsetID,
		ExtendedRequestID: requsetID,
		EventType:         "DISCONNECT",
		RouteKey:          "$disconnect",
		DomainName:        originReq.Host,
		Identity: events.APIGatewayRequestIdentity{
			SourceIP: originReq.RemoteAddr,
		},
		ConnectedAt:      int64(connectedAt.UnixNano() / int64(time.Millisecond)),
		RequestTime:      time.Now().Format(time.Layout),
		RequestTimeEpoch: int64(time.Now().UnixNano() / int64(time.Millisecond)),
		MessageDirection: "IN",
	}
	ctx := contextWithRequestContext(context.Background(), proxyCtx)
	ctx = contextWithCallbackURL(ctx, h.callbackURL)
	req, err := h.newBridgeRequest(
		ctx,
		connectionID,
		originReq.Host,
		"DISCONNECT",
		"$disconnect",
		bytes.NewReader([]byte{}),
	)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to create bridge request", "detail", err)
		return
	}
	req.Header.Set(HTTPHeaderConnectionID, connectionID)
	req.Header.Set(HTTPHeaderRequestID, requsetID)
	req.Header.Set(HTTPHeaderEventType, "DISCONNECT")
	req.Header.Set(HTTPHeaderRouteKey, "$disconnect")
	respWriter := NewResponseWriter()
	h.Handler.ServeHTTP(respWriter, req)
	if h.verbose {
		h.logger.Info(
			"disconnected",
			"connection_id", connectionID,
			"remote_addr", originReq.RemoteAddr,
			"host", originReq.Host,
			"current_connections", h.currentConnections(),
		)
	}
}

func (h *WebsocketHTTPBridgeHandler) onReceiveMessage(connectionID string, ws *websocket.Conn, msg []byte) error {
	requsetID, err := generateID(11)
	if err != nil {
		h.removeFromConnectionList(connectionID, websocket.CloseInternalServerErr, "failed to generate request id")
		return err
	}
	connectedAt, _, originReq := h.getConnectionInfo(connectionID)
	routeKey, err := h.routeKeySelector(msg)
	if err != nil {
		if h.verbose {
			h.logger.Warn("failed to select route key, fallback to $default", "detail", err, "connection_id", connectionID)
		}
		routeKey = "$default"
	}
	if routeKey == "" {
		if h.verbose {
			h.logger.Warn("route key is empty, fallback to $default", "connection_id", connectionID)
		}
		routeKey = "$default"
	}
	proxyCtx := events.APIGatewayWebsocketProxyRequestContext{
		ConnectionID:      connectionID,
		RequestID:         requsetID,
		ExtendedRequestID: requsetID,
		EventType:         "MESSAGE",
		RouteKey:          routeKey,
		DomainName:        originReq.Host,
		Identity: events.APIGatewayRequestIdentity{
			SourceIP: originReq.RemoteAddr,
		},
		ConnectedAt:      int64(connectedAt.UnixNano() / int64(time.Millisecond)),
		RequestTime:      time.Now().Format(time.Layout),
		RequestTimeEpoch: int64(time.Now().UnixNano() / int64(time.Millisecond)),
		MessageDirection: "IN",
	}
	ctx := contextWithRequestContext(context.Background(), proxyCtx)
	ctx = contextWithCallbackURL(ctx, h.callbackURL)
	req, err := h.newBridgeRequest(
		ctx,
		connectionID,
		originReq.Host,
		"MESSAGE",
		routeKey,
		bytes.NewReader(msg),
	)
	if err != nil {
		h.removeFromConnectionList(connectionID, websocket.CloseInternalServerErr, "failed to create bridge request")
		return err
	}
	req.Header.Set(HTTPHeaderConnectionID, connectionID)
	req.Header.Set(HTTPHeaderRequestID, requsetID)
	req.Header.Set(HTTPHeaderEventType, "MESSAGE")
	req.Header.Set(HTTPHeaderRouteKey, routeKey)
	respWriter := NewResponseWriter()
	h.Handler.ServeHTTP(respWriter, req)
	var messageType int
	if isBinary(respWriter.header) {
		messageType = websocket.BinaryMessage
	} else {
		messageType = websocket.TextMessage
	}
	if err := ws.WriteMessage(messageType, respWriter.Bytes()); err != nil {
		h.removeFromConnectionList(connectionID, websocket.CloseInternalServerErr, "failed to send message")
		return err
	}
	h.markActiveAt(connectionID)
	return nil
}
