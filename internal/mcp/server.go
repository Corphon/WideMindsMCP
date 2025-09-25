//MCP Server(MCP服务器)

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	appErrors "WideMindsMCP/internal/errors"
	"WideMindsMCP/internal/services"
)

// 结构体
type MCPServer struct {
	thoughtExpander *services.ThoughtExpander
	sessionManager  *services.SessionManager
	tools           map[string]MCPTool
	server          *http.Server
	mutex           sync.RWMutex
}

type MCPRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type MCPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 函数
func NewMCPServer(te *services.ThoughtExpander, sm *services.SessionManager) *MCPServer {
	return &MCPServer{
		thoughtExpander: te,
		sessionManager:  sm,
		tools:           make(map[string]MCPTool),
	}
}

// 方法
func (s *MCPServer) Start(port int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.server != nil {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleHTTP)
	mux.HandleFunc("/tools", s.handleTools)

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// log via utils logger to avoid panic
			// fall back to standard log if necessary
			fmt.Printf("MCP server error: %v\n", err)
		}
	}()

	return nil
}

func (s *MCPServer) HandleRequest(req *MCPRequest) *MCPResponse {
	if req == nil {
		return &MCPResponse{Error: &MCPError{Code: http.StatusBadRequest, Message: appErrors.ErrInvalidRequest.Error()}}
	}

	tool := s.getTool(req.Method)
	if tool == nil {
		return &MCPResponse{Error: &MCPError{Code: http.StatusNotFound, Message: appErrors.ErrToolNotFound.Error()}}
	}

	result, err := tool.Execute(req.Params)
	if err != nil {
		return &MCPResponse{Error: &MCPError{Code: statusFromError(err), Message: err.Error()}}
	}

	return &MCPResponse{Result: result}
}

func (s *MCPServer) RegisterTool(name string, tool MCPTool) {
	if tool == nil || name == "" {
		return
	}

	s.mutex.Lock()
	s.tools[name] = tool
	s.mutex.Unlock()
}

func (s *MCPServer) GetToolList() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	return names
}

func (s *MCPServer) Shutdown() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.server.Shutdown(ctx)
	s.server = nil
	return err
}

func (s *MCPServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, MCPResponse{Error: &MCPError{Code: http.StatusBadRequest, Message: err.Error()}})
		return
	}

	resp := s.HandleRequest(&req)
	respondJSON(w, *resp)
}

func (s *MCPServer) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp := MCPResponse{Result: s.GetToolList()}
	respondJSON(w, resp)
}

func (s *MCPServer) getTool(name string) MCPTool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.tools[name]
}

func statusFromError(err error) int {
	switch {
	case errors.Is(err, appErrors.ErrInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, appErrors.ErrSessionNotFound), errors.Is(err, appErrors.ErrToolNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func respondJSON(w http.ResponseWriter, resp MCPResponse) {
	w.Header().Set("Content-Type", "application/json")
	if resp.Error != nil && resp.Error.Code != 0 {
		w.WriteHeader(resp.Error.Code)
	}
	_ = json.NewEncoder(w).Encode(resp)
}
