package grpc

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// Client is a lightweight JSON-over-TCP RPC client.
type Client struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
	mu      sync.Mutex
	nextID  atomic.Int64
}

// Dial connects to an RPC server at the given address.
func Dial(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", addr, err)
	}
	return &Client{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
	}, nil
}

// Call invokes the named RPC method with params and decodes the response
// into result. Call is safe for concurrent use.
func (c *Client) Call(method string, params any, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)

	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshaling params: %w", err)
	}

	req := Request{
		Method: method,
		ID:     fmt.Sprintf("%d", id),
		Params: raw,
	}

	if err := c.encoder.Encode(req); err != nil {
		return fmt.Errorf("sending request: %w", err)
	}

	var resp Response
	if err := c.decoder.Decode(&resp); err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.Error != "" {
		return fmt.Errorf("rpc error: %s", resp.Error)
	}

	if result != nil {
		data, err := json.Marshal(resp.Data)
		if err != nil {
			return fmt.Errorf("marshaling response data: %w", err)
		}
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("unmarshaling into result: %w", err)
		}
	}

	return nil
}

// Close closes the underlying TCP connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
