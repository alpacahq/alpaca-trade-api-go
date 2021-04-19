package stream

import (
	"context"
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

	return &nhooyrWebsocketConn{
		conn:    conn,
		msgType: websocket.MessageBinary,
	}, err
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
