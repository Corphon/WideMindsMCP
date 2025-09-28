package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	appErrors "WideMindsMCP/internal/errors"
	"WideMindsMCP/internal/mcp"
	"WideMindsMCP/internal/models"
	"WideMindsMCP/internal/services"
	"WideMindsMCP/internal/storage"
	"WideMindsMCP/internal/utils"
)

// 结构体
type Config struct {
	Port                   int    `yaml:"port" json:"port"`
	MCPPort                int    `yaml:"mcp_port" json:"mcp_port"`
	LLMAPIKey              string `yaml:"llm_api_key" json:"llm_api_key"`
	LLMBaseURL             string `yaml:"llm_base_url" json:"llm_base_url"`
	LLMModel               string `yaml:"llm_model" json:"llm_model"`
	DataDir                string `yaml:"data_dir" json:"data_dir"`
	WebDir                 string `yaml:"web_dir" json:"web_dir"`
	UseFileStore           bool   `yaml:"use_file_store" json:"use_file_store"`
	APIToken               string `yaml:"api_token" json:"api_token"`
	HTTPRateLimitPerMinute int    `yaml:"http_rate_limit_per_minute" json:"http_rate_limit_per_minute"`
	MCPRateLimitPerMinute  int    `yaml:"mcp_rate_limit_per_minute" json:"mcp_rate_limit_per_minute"`
}

const (
	maxRequestBodyBytes int64 = 64 * 1024
)

// 函数
func main() {
	cfg, err := loadConfig()
	if err != nil {
		utils.Error("failed to load config", utils.KV("error", err))
		os.Exit(1)
	}

	thoughtExpander, sessionManager, llm, err := initializeServices(cfg)
	if err != nil {
		utils.Error("failed to initialize services", utils.KV("error", err))
		os.Exit(1)
	}

	mcpServer := setupMCPServer(cfg, thoughtExpander, sessionManager)
	if err := mcpServer.Start(cfg.MCPPort); err != nil {
		utils.Error("failed to start MCP server", utils.KV("error", err))
		os.Exit(1)
	}

	webMux := setupWebServer(cfg, sessionManager, thoughtExpander, llm)
	webServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           webMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		utils.Info("web server listening", utils.KV("addr", webServer.Addr))
		if err := webServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			utils.Error("web server error", utils.KV("error", err))
		}
	}()

	gracefulShutdown(mcpServer, webServer)
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		Port:                   8080,
		MCPPort:                9090,
		LLMModel:               "gpt-4.1",
		WebDir:                 "web",
		UseFileStore:           false,
		HTTPRateLimitPerMinute: 120,
		MCPRateLimitPerMinute:  60,
	}

	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	envPath := flag.String("env", "configs/example.env", "Path to env file")
	flag.Parse()

	if info, err := os.Stat(*envPath); err == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("env path %s is a directory", *envPath)
		}
		if _, err := utils.LoadEnvFile(*envPath); err != nil {
			return nil, fmt.Errorf("load env file %s: %w", *envPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat env file %s: %w", *envPath, err)
	}

	resolvedPath, err := utils.ResolveConfigPath(*configPath)
	if err == nil {
		if _, statErr := os.Stat(resolvedPath); statErr == nil {
			if err := utils.LoadYAML(resolvedPath, cfg); err != nil {
				return nil, err
			}
		}
	}

	applyEnvOverrides(cfg)

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if portStr := os.Getenv("PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = port
		}
	}
	if mcpPortStr := os.Getenv("MCP_PORT"); mcpPortStr != "" {
		if port, err := strconv.Atoi(mcpPortStr); err == nil {
			cfg.MCPPort = port
		}
	}
	if val := os.Getenv("LLM_API_KEY"); val != "" {
		cfg.LLMAPIKey = val
	}
	if val := os.Getenv("LLM_BASE_URL"); val != "" {
		cfg.LLMBaseURL = val
	}
	if val := os.Getenv("LLM_MODEL"); val != "" {
		cfg.LLMModel = val
	}
	if val := os.Getenv("DATA_DIR"); val != "" {
		cfg.DataDir = val
	}
	if val := os.Getenv("WEB_DIR"); val != "" {
		cfg.WebDir = val
	}
	if val := os.Getenv("USE_FILE_STORE"); val != "" {
		cfg.UseFileStore = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("API_TOKEN"); val != "" {
		cfg.APIToken = val
	}
	if val := os.Getenv("HTTP_RATE_LIMIT_PER_MINUTE"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil {
			cfg.HTTPRateLimitPerMinute = limit
		}
	}
	if val := os.Getenv("MCP_RATE_LIMIT_PER_MINUTE"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil {
			cfg.MCPRateLimitPerMinute = limit
		}
	}
}

func validateConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.Port)
	}
	if cfg.MCPPort <= 0 || cfg.MCPPort > 65535 {
		return fmt.Errorf("invalid mcp_port: %d", cfg.MCPPort)
	}
	if cfg.HTTPRateLimitPerMinute < 0 {
		return fmt.Errorf("invalid http_rate_limit_per_minute: %d", cfg.HTTPRateLimitPerMinute)
	}
	if cfg.MCPRateLimitPerMinute < 0 {
		return fmt.Errorf("invalid mcp_rate_limit_per_minute: %d", cfg.MCPRateLimitPerMinute)
	}
	if strings.TrimSpace(cfg.LLMBaseURL) != "" && strings.TrimSpace(cfg.LLMAPIKey) == "" {
		return errors.New("llm_api_key is required when llm_base_url is set; ensure the env file or config provides this value")
	}
	return nil
}

func initializeServices(config *Config) (*services.ThoughtExpander, *services.SessionManager, *services.LLMOrchestrator, error) {
	var sessionStore storage.SessionStore
	if config.UseFileStore || config.DataDir != "" {
		sessionStore = storage.NewFileSessionStore(config.DataDir)
	} else {
		sessionStore = storage.NewInMemorySessionStore()
	}

	sessionManager := services.NewSessionManager(sessionStore)
	llm := services.NewLLMOrchestrator(config.LLMAPIKey, config.LLMBaseURL, config.LLMModel)
	expander := services.NewThoughtExpander(llm, sessionManager)

	return expander, sessionManager, llm, nil
}

func setupMCPServer(cfg *Config, te *services.ThoughtExpander, sm *services.SessionManager) *mcp.MCPServer {
	server := mcp.NewMCPServer(te, sm, cfg.APIToken, cfg.MCPRateLimitPerMinute)
	server.RegisterTool("expand_thought", mcp.NewExpandThoughtTool(te))
	server.RegisterTool("explore_direction", mcp.NewExploreDirectionTool(te))
	server.RegisterTool("create_session", mcp.NewCreateSessionTool(sm))
	server.RegisterTool("get_session", mcp.NewGetSessionTool(sm))
	server.RegisterTool("list_sessions", mcp.NewListSessionsTool(sm))
	server.RegisterTool("delete_session", mcp.NewDeleteSessionTool(sm))
	server.RegisterTool("update_thought", mcp.NewUpdateThoughtTool(sm))
	server.RegisterTool("delete_thought", mcp.NewDeleteThoughtTool(sm))
	return server
}

func setupWebServer(cfg *Config, sessionManager *services.SessionManager, expander *services.ThoughtExpander, llm *services.LLMOrchestrator) *http.ServeMux {
	webDir := cfg.WebDir
	if webDir == "" {
		webDir = "web"
	}

	mux := http.NewServeMux()
	staticDir := filepath.Join(webDir, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		templatePath := filepath.Join(webDir, "templates", "mindmap.html")
		http.ServeFile(w, r, templatePath)
	})

	livenessResponder := func(w http.ResponseWriter, status string) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"status":    status,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(response)
	}

	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		livenessResponder(w, "ok")
	})

	readinessHandler := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		statusCode := http.StatusOK
		dependencies := map[string]string{}

		if sessionManager == nil {
			statusCode = http.StatusServiceUnavailable
			dependencies["session_store"] = "missing session manager"
		} else if err := sessionManager.HealthCheck(ctx); err != nil {
			statusCode = http.StatusServiceUnavailable
			dependencies["session_store"] = err.Error()
		} else {
			dependencies["session_store"] = "ok"
		}

		if llm == nil {
			statusCode = http.StatusServiceUnavailable
			dependencies["llm_orchestrator"] = "missing orchestrator"
		} else if err := llm.HealthCheck(ctx); err != nil {
			statusCode = http.StatusServiceUnavailable
			dependencies["llm_orchestrator"] = err.Error()
		} else {
			dependencies["llm_orchestrator"] = "ok"
		}

		statusLabel := "ok"
		if statusCode != http.StatusOK {
			statusLabel = "unavailable"
		}

		payload := map[string]interface{}{
			"status":       statusLabel,
			"dependencies": dependencies,
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(payload)
	}

	mux.HandleFunc("/healthz", readinessHandler)
	mux.HandleFunc("/readyz", readinessHandler)

	rateLimiter := utils.NewRateLimiter(cfg.HTTPRateLimitPerMinute, time.Minute)

	wrap := func(handler http.HandlerFunc, secure bool, limited bool) http.Handler {
		h := http.Handler(handler)
		if limited && rateLimiter != nil {
			next := h
			h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token := utils.ResolveRequestToken(r)
				key := utils.ClientKey(r, token)
				if !rateLimiter.Allow(key) {
					http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
					return
				}
				next.ServeHTTP(w, r)
			})
		}
		if secure && cfg.APIToken != "" {
			next := h
			h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token := utils.ResolveRequestToken(r)
				if token != cfg.APIToken {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			})
		}
		return h
	}

	mux.Handle("/api/sessions", wrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
			if userID == "" {
				respondError(w, utils.ValidationError("user_id is required"))
				return
			}
			if err := utils.ValidateUserID(userID); err != nil {
				respondError(w, err)
				return
			}
			sessions, err := sessionManager.ListSessions(userID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, sessions)
		case http.MethodPost:
			var payload struct {
				UserID  string `json:"user_id"`
				Concept string `json:"concept"`
			}
			if err := decodeJSONBody(w, r, &payload); err != nil {
				respondError(w, err)
				return
			}
			payload.UserID = strings.TrimSpace(payload.UserID)
			payload.Concept = strings.TrimSpace(payload.Concept)

			if err := utils.ValidateUserID(payload.UserID); err != nil {
				respondError(w, err)
				return
			}
			if err := utils.ValidateConcept(payload.Concept); err != nil {
				respondError(w, err)
				return
			}

			session, err := sessionManager.CreateSession(payload.UserID, payload.Concept)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, session)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}, true, true))

	mux.Handle("/api/sessions/", wrap(func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/sessions/"))
		if trimmed == "" {
			http.Error(w, "session id is required", http.StatusBadRequest)
			return
		}

		parts := make([]string, 0)
		for _, segment := range strings.Split(trimmed, "/") {
			if segment = strings.TrimSpace(segment); segment != "" {
				parts = append(parts, segment)
			}
		}
		if len(parts) == 0 {
			http.Error(w, "session id is required", http.StatusBadRequest)
			return
		}

		sessionID := parts[0]
		if err := utils.ValidateSessionID(sessionID); err != nil {
			respondError(w, err)
			return
		}

		if len(parts) >= 2 && parts[1] == "thoughts" {
			if len(parts) < 3 {
				http.Error(w, "thought id is required", http.StatusBadRequest)
				return
			}
			thoughtID := parts[2]
			switch r.Method {
			case http.MethodPatch:
				var payload models.ThoughtUpdate
				if err := decodeJSONBody(w, r, &payload); err != nil {
					respondError(w, err)
					return
				}
				if err := utils.ValidateThoughtUpdate(&payload); err != nil {
					respondError(w, err)
					return
				}
				thought, err := sessionManager.UpdateThought(sessionID, thoughtID, &payload)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, thought)
			case http.MethodDelete:
				session, err := sessionManager.DeleteThought(sessionID, thoughtID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, session)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		switch r.Method {
		case http.MethodGet:
			session, err := sessionManager.GetSession(sessionID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, session)
		case http.MethodPost:
			var payload struct {
				Direction models.Direction `json:"direction"`
			}
			if err := decodeJSONBody(w, r, &payload); err != nil {
				respondError(w, err)
				return
			}
			if err := utils.ValidateDirection(&payload.Direction); err != nil {
				respondError(w, err)
				return
			}
			thought, err := expander.ExploreDirection(payload.Direction, sessionID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, thought)
		case http.MethodDelete:
			if err := sessionManager.DeleteSession(sessionID); err != nil {
				respondError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}, true, true))

	mux.Handle("/api/expand", wrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Concept       string   `json:"concept"`
			Context       []string `json:"context"`
			ExpansionType string   `json:"expansion_type"`
		}
		if err := decodeJSONBody(w, r, &payload); err != nil {
			respondError(w, err)
			return
		}

		payload.Concept = strings.TrimSpace(payload.Concept)
		if err := utils.ValidateConcept(payload.Concept); err != nil {
			respondError(w, err)
			return
		}

		normalizedContext, err := utils.NormalizeContext(payload.Context)
		if err != nil {
			respondError(w, err)
			return
		}

		if trimmed := strings.TrimSpace(payload.ExpansionType); trimmed != "" {
			dirType, err := utils.ParseDirectionType(trimmed)
			if err != nil {
				respondError(w, err)
				return
			}
			payload.ExpansionType = string(dirType)
		} else {
			payload.ExpansionType = ""
		}

		result, err := expander.Expand(&services.ExpansionRequest{
			Concept:       payload.Concept,
			Context:       normalizedContext,
			ExpansionType: models.DirectionType(payload.ExpansionType),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, result)
	}, true, true))

	return mux
}

func gracefulShutdown(mcpServer *mcp.MCPServer, webServer *http.Server) {
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	<-shutdownCh
	utils.Warn("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := webServer.Shutdown(ctx); err != nil {
		utils.Error("failed to shutdown web server", utils.KV("error", err))
	}

	if err := mcpServer.Shutdown(); err != nil {
		utils.Error("failed to shutdown MCP server", utils.KV("error", err))
	}
}

func respondJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func respondError(w http.ResponseWriter, err error) {
	status := statusFromError(err)
	http.Error(w, err.Error(), status)
}

func statusFromError(err error) int {
	switch {
	case errors.Is(err, appErrors.ErrInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, appErrors.ErrSessionNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	if r == nil || r.Body == nil {
		return utils.ValidationError("request body is empty")
	}

	limited := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer limited.Close()

	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return utils.ValidationError("request body is empty")
		}
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return utils.ValidationError(fmt.Sprintf("request body contains badly-formed JSON (at position %d)", syntaxErr.Offset))
		}
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return utils.ValidationError(fmt.Sprintf("request body has an invalid value for %q (expected %s)", typeErr.Field, typeErr.Type))
		}
		return utils.ValidationError("request body is invalid")
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return utils.ValidationError("request body must contain a single JSON value")
	}

	return nil
}
