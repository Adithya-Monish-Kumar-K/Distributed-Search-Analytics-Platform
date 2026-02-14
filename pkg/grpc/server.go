// Package grpc provides a lightweight JSON-over-TCP RPC framework
// for internal service-to-service communication.
//
// This is a custom implementation that avoids the full google.golang.org/grpc
// dependency while providing the core RPC patterns: service registration,
// method dispatch, request/response framing, and client connection pooling.
//
// Protocol: newline-delimited JSON over a persistent TCP connection.
//
// Example server:
//
//	s := grpc.NewServer()
//	s.Register("SearchService.Search", func(ctx context.Context, req json.RawMessage) (any, error) {
//	    var searchReq proto.SearchRequest
//	    json.Unmarshal(req, &searchReq)
//	    // ... execute search ...
//	    return &proto.SearchResponse{...}, nil
//	})
//	s.Serve(":9000")
//
// Example client:
//
//	c, _ := grpc.Dial("localhost:9000")
//	var resp proto.SearchResponse
//	c.Call("SearchService.Search", &proto.SearchRequest{Query: "hello"}, &resp)
package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// HandlerFunc processes an RPC request and returns a response or error.
type HandlerFunc func(ctx context.Context, req json.RawMessage) (any, error)

// Request is the wire format for an RPC request.
type Request struct {
	Method string          `json:"method"`
	ID     string          `json:"id"`
	Params json.RawMessage `json:"params"`
}

// Response is the wire format for an RPC response.
type Response struct {
	ID    string `json:"id"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// Server is a lightweight JSON-over-TCP RPC server.
type Server struct {
	handlers map[string]HandlerFunc
	listener net.Listener
	logger   *slog.Logger
	mu       sync.RWMutex
	wg       sync.WaitGroup
	done     chan struct{}
}

// NewServer creates a new RPC server.
func NewServer() *Server {
	return &Server{
		handlers: make(map[string]HandlerFunc),
		logger:   slog.Default().With("component", "rpc-server"),
		done:     make(chan struct{}),
	}
}

// Register adds a handler for the given RPC method name.
// Method names follow the "Service.Method" convention.
func (s *Server) Register(method string, handler HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
	s.logger.Debug("method registered", "method", method)
}

// Serve starts accepting TCP connections on the given address.
// It blocks until Stop is called.
func (s *Server) Serve(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	s.listener = ln
	s.logger.Info("rpc server listening", "addr", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				s.logger.Error("accept error", "error", err)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			return // connection closed or read error
		}

		s.mu.RLock()
		handler, exists := s.handlers[req.Method]
		s.mu.RUnlock()

		resp := Response{ID: req.ID}

		if !exists {
			resp.Error = fmt.Sprintf("unknown method: %s", req.Method)
		} else {
			data, err := handler(context.Background(), req.Params)
			if err != nil {
				resp.Error = err.Error()
			} else {
				resp.Data = data
			}
		}

		if err := encoder.Encode(resp); err != nil {
			s.logger.Error("write error", "method", req.Method, "error", err)
			return
		}
	}
}

// MethodCount returns the number of registered methods.
func (s *Server) MethodCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.handlers)
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	close(s.done)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	s.logger.Info("rpc server stopped")
}
