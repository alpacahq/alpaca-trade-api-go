package stream

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
)

type coderWebsocketConn struct {
	conn    *websocket.Conn
	msgType websocket.MessageType
}

// newCoderWebsocketConn creates a new coder websocket connection
func newCoderWebsocketConn(ctx context.Context, u url.URL) (conn, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	reqHeader := http.Header{}
	reqHeader.Set("Content-Type", "application/msgpack")
	reqHeader.Set("User-Agent", alpaca.Version())
	//nolint:bodyclose // According to its docs: you never need to close resp.Body yourself
	conn, _, err := websocket.Dial(ctxWithTimeout, u.String(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		HTTPHeader:      reqHeader,
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	// Disable read limit: especially news messages can be huge.
	conn.SetReadLimit(-1)

	return &coderWebsocketConn{
		conn:    conn,
		msgType: websocket.MessageBinary,
	}, nil
}

// close closes the websocket connection
func (c *coderWebsocketConn) close() error {
	return c.conn.Close(websocket.StatusNormalClosure, "")
}

// ping sends a ping to the client
func (c *coderWebsocketConn) ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, pongWait)
	defer cancel()

	return c.conn.Ping(pingCtx)
}

// readMessage blocks until it reads a single message
func (c *coderWebsocketConn) readMessage(ctx context.Context) ([]byte, error) {
	_, data, err := c.conn.Read(ctx)
	return data, err
}

// writeMessage writes a single message
func (c *coderWebsocketConn) writeMessage(ctx context.Context, data []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, writeWait)
	defer cancel()

	return c.conn.Write(writeCtx, c.msgType, data)
}
