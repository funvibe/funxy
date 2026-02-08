package evaluator

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/funvibe/funbit/pkg/funbit"
)

// WebSocket opcodes
const (
	wsOpContinuation = 0
	wsOpText         = 1
	wsOpBinary       = 2
	wsOpClose        = 8
	wsOpPing         = 9
	wsOpPong         = 10
)

// wsConnection represents a WebSocket connection
type wsConnection struct {
	conn     net.Conn
	isClient bool
	reader   *bufio.Reader
	writeMu  sync.Mutex
	closed   bool
	closeMu  sync.Mutex
}

// Global WebSocket connection storage
var (
	wsConnections   = make(map[int64]*wsConnection)
	wsConnectionsMu sync.RWMutex
	wsNextConnID    int64 = 1
)

// Global WebSocket server storage
var (
	wsServers   = make(map[int64]*wsServer)
	wsServersMu sync.RWMutex
	wsNextSrvID int64 = 1
)

type wsServer struct {
	listener net.Listener
	shutdown chan struct{}
	handler  Object
	eval     *Evaluator
	running  bool
}

// WsBuiltins returns WebSocket built-in functions
func WsBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"wsConnect":        {Fn: builtinWsConnect, Name: "wsConnect"},
		"wsConnectTimeout": {Fn: builtinWsConnectTimeout, Name: "wsConnectTimeout"},
		"wsSend":           {Fn: builtinWsSend, Name: "wsSend"},
		"wsRecv":           {Fn: builtinWsRecv, Name: "wsRecv"},
		"wsRecvTimeout":    {Fn: builtinWsRecvTimeout, Name: "wsRecvTimeout"},
		"wsClose":          {Fn: builtinWsClose, Name: "wsClose"},
		"wsServe":          {Fn: builtinWsServe, Name: "wsServe"},
		"wsServeAsync":     {Fn: builtinWsServeAsync, Name: "wsServeAsync"},
		"wsServerStop":     {Fn: builtinWsServerStop, Name: "wsServerStop"},
	}
}

// SetWsBuiltinTypes sets type information for WebSocket builtins
func SetWsBuiltinTypes(builtins map[string]*Builtin) {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Result types
	// Result<String, Int>
	resultInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Int},
	}
	// Result<String, Nil>
	resultNil := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Nil},
	}
	// Result<String, String>
	resultString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, stringType},
	}

	// Option<String>
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}

	// Result<String, Option<String>>
	resultOptionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, optionString},
	}

	// Handler type: (Int, String) -> String
	handlerType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.Int, stringType},
		ReturnType: stringType,
	}

	types := map[string]typesystem.Type{
		"wsConnect":        typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultInt},
		"wsConnectTimeout": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int}, ReturnType: resultInt},
		"wsSend":           typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, stringType}, ReturnType: resultNil},
		"wsRecv":           typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultString},
		"wsRecvTimeout":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: resultOptionString},
		"wsClose":          typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultNil},
		"wsServe":          typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, handlerType}, ReturnType: resultNil},
		"wsServeAsync":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, handlerType}, ReturnType: resultInt},
		"wsServerStop":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultNil},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// builtinWsConnect connects to a WebSocket server
// wsConnect(url: String) -> Result<String, Int>
func builtinWsConnect(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("wsConnect requires 1 argument (url)")
	}

	urlList, ok := args[0].(*List)
	if !ok {
		return newError("wsConnect: url must be a String")
	}

	urlStr := listToString(urlList)
	conn, err := wsDialWithTimeout(urlStr, 30*time.Second)
	if err != nil {
		return makeFailStr(err.Error())
	}

	wsConnectionsMu.Lock()
	connID := wsNextConnID
	wsNextConnID++
	wsConnections[connID] = conn
	wsConnectionsMu.Unlock()

	return makeOk(&Integer{Value: connID})
}

// builtinWsConnectTimeout connects with custom timeout
// wsConnectTimeout(url: String, timeoutMs: Int) -> Result<String, Int>
func builtinWsConnectTimeout(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("wsConnectTimeout requires 2 arguments (url, timeoutMs)")
	}

	urlList, ok := args[0].(*List)
	if !ok {
		return newError("wsConnectTimeout: url must be a String")
	}

	timeoutMs, ok := args[1].(*Integer)
	if !ok {
		return newError("wsConnectTimeout: timeoutMs must be an Int")
	}

	urlStr := listToString(urlList)
	conn, err := wsDialWithTimeout(urlStr, time.Duration(timeoutMs.Value)*time.Millisecond)
	if err != nil {
		return makeFailStr(err.Error())
	}

	wsConnectionsMu.Lock()
	connID := wsNextConnID
	wsNextConnID++
	wsConnections[connID] = conn
	wsConnectionsMu.Unlock()

	return makeOk(&Integer{Value: connID})
}

// wsDialWithTimeout performs WebSocket handshake with timeout
func wsDialWithTimeout(urlStr string, timeout time.Duration) (*wsConnection, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	// Determine host and port
	host := parsed.Host
	if !strings.Contains(host, ":") {
		if parsed.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// Connect with timeout
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %v", err)
	}

	// Generate WebSocket key
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	wsKey := base64.StdEncoding.EncodeToString(key)

	// Build and send handshake request
	path := parsed.Path
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}

	request := fmt.Sprintf("GET %s HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\n"+
		"Sec-WebSocket-Version: 13\r\n"+
		"\r\n", path, parsed.Host, wsKey)

	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte(request)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake write failed: %v", err)
	}

	// Read response
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	reader := bufio.NewReader(conn)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake read failed: %v", err)
	}

	if !strings.Contains(statusLine, "101") {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake failed: %s", strings.TrimSpace(statusLine))
	}

	// Read headers until empty line
	expectedAccept := computeAcceptKey(wsKey)
	gotAccept := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("handshake header read failed: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "sec-websocket-accept:") {
			accept := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			if accept == expectedAccept {
				gotAccept = true
			}
		}
	}

	if !gotAccept {
		_ = conn.Close()
		return nil, fmt.Errorf("invalid Sec-WebSocket-Accept")
	}

	// Clear deadlines
	_ = conn.SetDeadline(time.Time{})

	return &wsConnection{
		conn:     conn,
		isClient: true,
		reader:   reader,
	}, nil
}

// computeAcceptKey computes the Sec-WebSocket-Accept value
func computeAcceptKey(key string) string {
	const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// builtinWsSend sends a text message
// wsSend(connId: Int, message: String) -> Result<String, Nil>
func builtinWsSend(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("wsSend requires 2 arguments (connId, message)")
	}

	connID, ok := args[0].(*Integer)
	if !ok {
		return newError("wsSend: connId must be an Int")
	}

	msgList, ok := args[1].(*List)
	if !ok {
		return newError("wsSend: message must be a String")
	}

	wsConnectionsMu.RLock()
	conn, exists := wsConnections[connID.Value]
	wsConnectionsMu.RUnlock()

	if !exists {
		return makeFailStr("connection not found")
	}

	message := listToString(msgList)
	if err := wsSendFrame(conn, wsOpText, []byte(message)); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// wsSendFrame sends a WebSocket frame using funbit
func wsSendFrame(ws *wsConnection, opcode int, payload []byte) error {
	ws.closeMu.Lock()
	if ws.closed {
		ws.closeMu.Unlock()
		return fmt.Errorf("connection closed")
	}
	ws.closeMu.Unlock()

	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()

	// Build frame using funbit
	builder := funbit.NewBuilder()

	// FIN=1, RSV1-3=0, opcode
	funbit.AddInteger(builder, 1, funbit.WithSize(1))             // FIN
	funbit.AddInteger(builder, 0, funbit.WithSize(3))             // RSV1-3
	funbit.AddInteger(builder, int64(opcode), funbit.WithSize(4)) // opcode

	// MASK (client must mask, server must not)
	mask := 0
	if ws.isClient {
		mask = 1
	}
	funbit.AddInteger(builder, int64(mask), funbit.WithSize(1))

	// Payload length
	payloadLen := len(payload)
	if payloadLen <= 125 {
		funbit.AddInteger(builder, int64(payloadLen), funbit.WithSize(7))
	} else if payloadLen <= 65535 {
		funbit.AddInteger(builder, 126, funbit.WithSize(7))
		funbit.AddInteger(builder, int64(payloadLen), funbit.WithSize(16), funbit.WithEndianness("big"))
	} else {
		funbit.AddInteger(builder, 127, funbit.WithSize(7))
		funbit.AddInteger(builder, int64(payloadLen), funbit.WithSize(64), funbit.WithEndianness("big"))
	}

	// Masking key (if client)
	var maskKey []byte
	if ws.isClient {
		maskKey = make([]byte, 4)
		_, _ = rand.Read(maskKey)
		funbit.AddBinary(builder, maskKey)
	}

	bitstring, err := funbit.Build(builder)
	if err != nil {
		return fmt.Errorf("frame build failed: %v", err)
	}

	// Write header
	if _, err := ws.conn.Write(bitstring.ToBytes()); err != nil {
		return fmt.Errorf("write failed: %v", err)
	}

	// Write payload (masked if client)
	if ws.isClient && len(maskKey) > 0 {
		maskedPayload := make([]byte, len(payload))
		for i := range payload {
			maskedPayload[i] = payload[i] ^ maskKey[i%4]
		}
		if _, err := ws.conn.Write(maskedPayload); err != nil {
			return fmt.Errorf("write payload failed: %v", err)
		}
	} else {
		if _, err := ws.conn.Write(payload); err != nil {
			return fmt.Errorf("write payload failed: %v", err)
		}
	}

	return nil
}

// builtinWsRecv receives a message (blocking)
// wsRecv(connId: Int) -> Result<String, String>
func builtinWsRecv(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("wsRecv requires 1 argument (connId)")
	}

	connID, ok := args[0].(*Integer)
	if !ok {
		return newError("wsRecv: connId must be an Int")
	}

	wsConnectionsMu.RLock()
	conn, exists := wsConnections[connID.Value]
	wsConnectionsMu.RUnlock()

	if !exists {
		return makeFailStr("connection not found")
	}

	// Clear deadline for blocking read
	_ = conn.conn.SetReadDeadline(time.Time{})

	message, err := wsReadMessage(conn)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(stringToList(message))
}

// builtinWsRecvTimeout receives with timeout
// wsRecvTimeout(connId: Int, timeoutMs: Int) -> Result<String, Option<String>>
func builtinWsRecvTimeout(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("wsRecvTimeout requires 2 arguments (connId, timeoutMs)")
	}

	connID, ok := args[0].(*Integer)
	if !ok {
		return newError("wsRecvTimeout: connId must be an Int")
	}

	timeoutMs, ok := args[1].(*Integer)
	if !ok {
		return newError("wsRecvTimeout: timeoutMs must be an Int")
	}

	wsConnectionsMu.RLock()
	conn, exists := wsConnections[connID.Value]
	wsConnectionsMu.RUnlock()

	if !exists {
		return makeFailStr("connection not found")
	}

	// Set read deadline
	_ = conn.conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs.Value) * time.Millisecond))

	message, err := wsReadMessage(conn)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// Timeout -> return Ok(None)
			return makeOk(makeNone())
		}
		return makeFailStr(err.Error())
	}

	// Return Ok(Some(message))
	return makeOk(makeSome(stringToList(message)))
}

// wsReadMessage reads a complete WebSocket message using funbit
func wsReadMessage(ws *wsConnection) (string, error) {
	ws.closeMu.Lock()
	if ws.closed {
		ws.closeMu.Unlock()
		return "", fmt.Errorf("connection closed")
	}
	ws.closeMu.Unlock()

	var messageData []byte

	for {
		// Read first 2 bytes (header)
		header := make([]byte, 2)
		if _, err := io.ReadFull(ws.reader, header); err != nil {
			return "", fmt.Errorf("read header failed: %v", err)
		}

		// Parse header using funbit
		headerBits := funbit.NewBitStringFromBytes(header)

		matcher := funbit.NewMatcher()
		var fin, rsv, opcode, mask, payloadLen7 int

		funbit.Integer(matcher, &fin, funbit.WithSize(1))
		funbit.Integer(matcher, &rsv, funbit.WithSize(3))
		funbit.Integer(matcher, &opcode, funbit.WithSize(4))
		funbit.Integer(matcher, &mask, funbit.WithSize(1))
		funbit.Integer(matcher, &payloadLen7, funbit.WithSize(7))

		results, err := funbit.Match(matcher, headerBits)
		if err != nil || len(results) == 0 {
			return "", fmt.Errorf("match header failed: %v", err)
		}

		// Handle extended payload length
		var payloadLen int64
		if payloadLen7 <= 125 {
			payloadLen = int64(payloadLen7)
		} else if payloadLen7 == 126 {
			extLen := make([]byte, 2)
			if _, err := io.ReadFull(ws.reader, extLen); err != nil {
				return "", fmt.Errorf("read ext len failed: %v", err)
			}
			extBits := funbit.NewBitStringFromBytes(extLen)
			extMatcher := funbit.NewMatcher()
			var extPayloadLen int
			funbit.Integer(extMatcher, &extPayloadLen, funbit.WithSize(16), funbit.WithEndianness("big"))
			_, _ = funbit.Match(extMatcher, extBits)
			payloadLen = int64(extPayloadLen)
		} else { // 127
			extLen := make([]byte, 8)
			if _, err := io.ReadFull(ws.reader, extLen); err != nil {
				return "", fmt.Errorf("read ext len failed: %v", err)
			}
			extBits := funbit.NewBitStringFromBytes(extLen)
			extMatcher := funbit.NewMatcher()
			funbit.Integer(extMatcher, &payloadLen, funbit.WithSize(64), funbit.WithEndianness("big"))
			_, _ = funbit.Match(extMatcher, extBits)
		}

		// Read masking key if present
		var maskKey []byte
		if mask == 1 {
			maskKey = make([]byte, 4)
			if _, err := io.ReadFull(ws.reader, maskKey); err != nil {
				return "", fmt.Errorf("read mask key failed: %v", err)
			}
		}

		// Read payload
		payload := make([]byte, payloadLen)
		if payloadLen > 0 {
			if _, err := io.ReadFull(ws.reader, payload); err != nil {
				return "", fmt.Errorf("read payload failed: %v", err)
			}
		}

		// Unmask if needed
		if mask == 1 && len(maskKey) > 0 {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}

		// Handle different opcodes
		switch opcode {
		case wsOpText, wsOpBinary, wsOpContinuation:
			messageData = append(messageData, payload...)
			if fin == 1 {
				return string(messageData), nil
			}
		case wsOpClose:
			ws.closeMu.Lock()
			ws.closed = true
			ws.closeMu.Unlock()
			return "", fmt.Errorf("connection closed by peer")
		case wsOpPing:
			// Respond with pong
			_ = wsSendFrame(ws, wsOpPong, payload)
		case wsOpPong:
			// Ignore pong
		}
	}
}

// builtinWsClose closes a WebSocket connection
// wsClose(connId: Int) -> Result<String, Nil>
func builtinWsClose(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("wsClose requires 1 argument (connId)")
	}

	connID, ok := args[0].(*Integer)
	if !ok {
		return newError("wsClose: connId must be an Int")
	}

	wsConnectionsMu.Lock()
	conn, exists := wsConnections[connID.Value]
	if exists {
		delete(wsConnections, connID.Value)
	}
	wsConnectionsMu.Unlock()

	if !exists {
		return makeFailStr("connection not found")
	}

	// Send close frame
	_ = wsSendFrame(conn, wsOpClose, nil)
	_ = conn.conn.Close()

	conn.closeMu.Lock()
	conn.closed = true
	conn.closeMu.Unlock()

	return makeOk(&Nil{})
}

// builtinWsServe starts a blocking WebSocket server
// wsServe(port: Int, handler: (connId: Int, message: String) -> String) -> Result<String, Nil>
func builtinWsServe(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("wsServe requires 2 arguments (port, handler)")
	}

	port, ok := args[0].(*Integer)
	if !ok {
		return newError("wsServe: port must be an Int")
	}

	handler := args[1]
	// Capture handler if CaptureHandler is available
	if e.CaptureHandler != nil {
		handler = e.CaptureHandler(handler)
	}

	if !wsIsCallable(handler) {
		return newError("wsServe: handler must be a function")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port.Value))
	if err != nil {
		return makeFailStr(fmt.Sprintf("listen failed: %v", err))
	}
	defer func() { _ = listener.Close() }()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		// Create a fresh evaluator/VM for each connection
		var connEval *Evaluator
		if e.Fork != nil {
			connEval = e.Fork()
		} else {
			connEval = e.Clone()
		}

		go handleWsConnection(conn, handler, connEval)
	}
}

// builtinWsServeAsync starts a non-blocking WebSocket server
// wsServeAsync(port: Int, handler: (connId: Int, message: String) -> String) -> Result<String, Int>
func builtinWsServeAsync(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("wsServeAsync requires 2 arguments (port, handler)")
	}

	port, ok := args[0].(*Integer)
	if !ok {
		return newError("wsServeAsync: port must be an Int")
	}

	handler := args[1]
	// Capture handler if CaptureHandler is available (VM mode)
	// This ensures upvalues are closed and safe for async execution
	if e.CaptureHandler != nil {
		handler = e.CaptureHandler(handler)
	}

	if !wsIsCallable(handler) {
		return newError("wsServeAsync: handler must be a function")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port.Value))
	if err != nil {
		return makeFailStr(fmt.Sprintf("listen failed: %v", err))
	}

	srv := &wsServer{
		listener: listener,
		shutdown: make(chan struct{}),
		handler:  handler,
		eval:     e,
		running:  true,
	}

	wsServersMu.Lock()
	srvID := wsNextSrvID
	wsNextSrvID++
	wsServers[srvID] = srv
	wsServersMu.Unlock()

	go func() {
		for {
			select {
			case <-srv.shutdown:
				return
			default:
				_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
				conn, err := listener.Accept()
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return
				}

				// Create a fresh evaluator/VM for each connection to ensure thread safety
				var connEval *Evaluator
				if srv.eval.Fork != nil {
					connEval = srv.eval.Fork()
				} else {
					connEval = srv.eval.Clone()
				}

				go handleWsConnection(conn, srv.handler, connEval)
			}
		}
	}()

	return makeOk(&Integer{Value: srvID})
}

// builtinWsServerStop stops a WebSocket server
// wsServerStop(serverId: Int) -> Result<String, Nil>
func builtinWsServerStop(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("wsServerStop requires 1 argument (serverId)")
	}

	srvID, ok := args[0].(*Integer)
	if !ok {
		return newError("wsServerStop: serverId must be an Int")
	}

	wsServersMu.Lock()
	srv, exists := wsServers[srvID.Value]
	if exists {
		delete(wsServers, srvID.Value)
	}
	wsServersMu.Unlock()

	if !exists {
		return makeFailStr("server not found")
	}

	close(srv.shutdown)
	_ = srv.listener.Close()
	srv.running = false

	return makeOk(&Nil{})
}

// handleWsConnection handles a single WebSocket connection
func handleWsConnection(conn net.Conn, handler Object, eval *Evaluator) {
	defer func() { _ = conn.Close() }()

	// Perform server-side handshake
	reader := bufio.NewReader(conn)

	// Read HTTP request
	requestLine, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(requestLine, "GET") {
		return
	}

	// Read headers
	var wsKey string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "sec-websocket-key:") {
			wsKey = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}

	if wsKey == "" {
		return
	}

	// Send handshake response
	accept := computeAcceptKey(wsKey)
	response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Accept: %s\r\n"+
		"\r\n", accept)

	_, _ = conn.Write([]byte(response))

	// Create server-side connection
	ws := &wsConnection{
		conn:     conn,
		isClient: false,
		reader:   reader,
	}

	// Store connection
	wsConnectionsMu.Lock()
	connID := wsNextConnID
	wsNextConnID++
	wsConnections[connID] = ws
	wsConnectionsMu.Unlock()

	defer func() {
		wsConnectionsMu.Lock()
		delete(wsConnections, connID)
		wsConnectionsMu.Unlock()
	}()

	// Message loop
	for {
		message, err := wsReadMessage(ws)
		if err != nil {
			break
		}

		// Call handler: (connId, message) -> response
		resp := eval.ApplyFunction(handler, []Object{
			&Integer{Value: connID},
			stringToList(message),
		})

		if resp != nil {
			if respList, ok := resp.(*List); ok {
				respStr := listToString(respList)
				if respStr != "" {
					_ = wsSendFrame(ws, wsOpText, []byte(respStr))
				}
			}
		}
	}
}

// Helper to check if object is callable
func wsIsCallable(obj Object) bool {
	switch obj.(type) {
	case *Function, *Builtin, *PartialApplication:
		return true
	}
	// Check for VM closure by type string
	if obj.Type() == "CLOSURE" || obj.Type() == "BUILTIN_CLOSURE" {
		return true
	}
	return false
}
