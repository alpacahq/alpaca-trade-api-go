package stream

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"time"

	"nhooyr.io/websocket"
)

type nhooyrWebsocketConn struct {
	conn       *websocket.Conn
	msgType    websocket.MessageType
	comression websocket.CompressionMode
}

// newNhooyrWebsocketConn creates a new nhooyr websocket connection
func newNhooyrWebsocketConn(ctx context.Context, u url.URL) (conn, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var compression websocket.CompressionMode
	compressionMode := os.Getenv("STREAM_COMPRESSION")
	switch compressionMode {
	case "no":
		compression = websocket.CompressionDisabled
	case "compression":
		compression = websocket.CompressionNoContextTakeover
	case "takeover":
		compression = websocket.CompressionContextTakeover
	default:
		compression = websocket.CompressionContextTakeover
	}
	conn, _, err := websocket.Dial(ctxWithTimeout, u.String(), &websocket.DialOptions{
		CompressionMode: compression,
		HTTPHeader: http.Header{
			"Content-Type": []string{"application/msgpack"},
		},
	})

	return &nhooyrWebsocketConn{
		conn:       conn,
		msgType:    websocket.MessageBinary,
		comression: compression,
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
