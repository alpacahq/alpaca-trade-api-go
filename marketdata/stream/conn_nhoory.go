package stream

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"nhooyr.io/websocket"
)

type nhooyrWebsocketConn struct {
	conn    *websocket.Conn
	msgType websocket.MessageType
}

// newNhooyrWebsocketConn creates a new nhooyr websocket connection
func newNhooyrWebsocketConn(ctx context.Context, u url.URL) (conn, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctxWithTimeout, u.String(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		HTTPHeader: http.Header{
			"Content-Type": []string{"application/msgpack"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	// If the client receives a message larger than the read limit, the read will fail and the
	// connection will be restarted.
	// The normal messages (trade, quotes, etc.) are well under the 64 kB websocket single frame limit,
	// however an unlimited user can subscribe to many symbols (in multiple subscribe calls),
	// and the server always returns ALL the subscribed symbols.
	// Increasing the read limit should not have a negative affect on performance or anything,
	// but it makes possible to read these large messages.
	conn.SetReadLimit(1024 * 1024)

	return &nhooyrWebsocketConn{
		conn:    conn,
		msgType: websocket.MessageBinary,
	}, nil
}

// close closes the websocket connection
func (c *nhooyrWebsocketConn) close() error {
	return c.conn.Close(websocket.StatusNormalClosure, "")
}

// ping sends a ping to the client
func (c *nhooyrWebsocketConn) ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, pongWait)
	defer cancel()

	return c.conn.Ping(pingCtx)
}

// readMessage blocks until it reads a single message
func (c *nhooyrWebsocketConn) readMessage(ctx context.Context) (data []byte, err error) {
	_, data, err = c.conn.Read(ctx)
	return data, err
}

// writeMessage writes a single message
func (c *nhooyrWebsocketConn) writeMessage(ctx context.Context, data []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, writeWait)
	defer cancel()

	return c.conn.Write(writeCtx, c.msgType, data)
}
