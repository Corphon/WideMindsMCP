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
	"unicode/utf8"

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
	maxRequestBodyBytes     int64 = 64 * 1024
	maxConceptLength              = 200
	maxUserIDLength               = 64
	maxSessionIDLength            = 64
	maxDirectionTitleLength       = 120
	maxDirectionDescLength        = 600
	maxKeywordLength              = 50
	maxDirectionKeywords          = 16
	maxContextItems               = 20
	maxContextItemLength          = 120
)

var allowedDirectionTypes = map[models.DirectionType]struct{}{
	models.Broad:    {},
	models.Deep:     {},
	models.Lateral:  {},
	models.Critical: {},
}

// 函数
func main() {
	cfg, err := loadConfig()
	if err != nil {
		utils.Errorf("failed to load config: %v", err)
		os.Exit(1)
	}

	thoughtExpander, sessionManager, err := initializeServices(cfg)
	if err != nil {
		utils.Errorf("failed to initialize services: %v", err)
		os.Exit(1)
	}

	mcpServer := setupMCPServer(cfg, thoughtExpander, sessionManager)
	if err := mcpServer.Start(cfg.MCPPort); err != nil {
		utils.Errorf("failed to start MCP server: %v", err)
		os.Exit(1)
	}

	webMux := setupWebServer(cfg, sessionManager, thoughtExpander)
	webServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           webMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		utils.Infof("web server listening on %s", webServer.Addr)
		if err := webServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			utils.Errorf("web server error: %v", err)
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

	if _, err := os.Stat(*envPath); err == nil {
		if _, err := utils.LoadEnvFile(*envPath); err != nil {
			utils.Warnf("failed to load env file: %v", err)
		}
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

func initializeServices(config *Config) (*services.ThoughtExpander, *services.SessionManager, error) {
	var sessionStore storage.SessionStore
	if config.UseFileStore || config.DataDir != "" {
		sessionStore = storage.NewFileSessionStore(config.DataDir)
	} else {
		sessionStore = storage.NewInMemorySessionStore()
	}

	sessionManager := services.NewSessionManager(sessionStore)
	llm := services.NewLLMOrchestrator(config.LLMAPIKey, config.LLMBaseURL, config.LLMModel)
	expander := services.NewThoughtExpander(llm, sessionManager)

	return expander, sessionManager, nil
}

func setupMCPServer(cfg *Config, te *services.ThoughtExpander, sm *services.SessionManager) *mcp.MCPServer {
	server := mcp.NewMCPServer(te, sm, cfg.APIToken, cfg.MCPRateLimitPerMinute)
	server.RegisterTool("expand_thought", mcp.NewExpandThoughtTool(te))
	server.RegisterTool("explore_direction", mcp.NewExploreDirectionTool(te))
	server.RegisterTool("create_session", mcp.NewCreateSessionTool(sm))
	server.RegisterTool("get_session", mcp.NewGetSessionTool(sm))
	return server
}

func setupWebServer(cfg *Config, sessionManager *services.SessionManager, expander *services.ThoughtExpander) *http.ServeMux {
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

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

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

			if err := validateUserID(payload.UserID); err != nil {
				respondError(w, err)
				return
			}
			if err := validateConcept(payload.Concept); err != nil {
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
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/sessions/"))
		if err := validateSessionID(id); err != nil {
			respondError(w, err)
			return
		}
		switch r.Method {
		case http.MethodGet:
			session, err := sessionManager.GetSession(id)
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
			if err := validateDirection(&payload.Direction); err != nil {
				respondError(w, err)
				return
			}
			thought, err := expander.ExploreDirection(payload.Direction, id)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, thought)
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
		if err := validateConcept(payload.Concept); err != nil {
			respondError(w, err)
			return
		}

		normalizedContext, err := normalizeContext(payload.Context)
		if err != nil {
			respondError(w, err)
			return
		}

		expansionType := strings.ToLower(strings.TrimSpace(payload.ExpansionType))
		if expansionType != "" {
			dirType := models.DirectionType(expansionType)
			if _, ok := allowedDirectionTypes[dirType]; !ok {
				respondError(w, fmt.Errorf("%w: invalid expansion_type", appErrors.ErrInvalidRequest))
				return
			}
			payload.ExpansionType = expansionType
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
	utils.Warnf("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := webServer.Shutdown(ctx); err != nil {
		utils.Errorf("failed to shutdown web server: %v", err)
	}

	if err := mcpServer.Shutdown(); err != nil {
		utils.Errorf("failed to shutdown MCP server: %v", err)
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
		return validationError("request body is empty")
	}

	limited := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer limited.Close()

	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return validationError("request body is empty")
		}
		return fmt.Errorf("%w: %v", appErrors.ErrInvalidRequest, err)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return validationError("request body must contain a single JSON value")
	}

	return nil
}

func validateConcept(concept string) error {
	if concept == "" {
		return validationError("concept is required")
	}
	if utf8.RuneCountInString(concept) > maxConceptLength {
		return validationError("concept is too long")
	}
	return nil
}

func validateUserID(userID string) error {
	if userID == "" {
		return nil
	}
	if strings.ContainsAny(userID, " \t\r\n") {
		return validationError("user_id must not contain whitespace")
	}
	if utf8.RuneCountInString(userID) > maxUserIDLength {
		return validationError("user_id is too long")
	}
	return nil
}

func validateSessionID(id string) error {
	if id == "" {
		return validationError("session_id is required")
	}
	if strings.ContainsAny(id, " \t\r\n") {
		return validationError("session_id must not contain whitespace")
	}
	if utf8.RuneCountInString(id) > maxSessionIDLength {
		return validationError("session_id is too long")
	}
	return nil
}

func normalizeContext(items []string) ([]string, error) {
	if len(items) > maxContextItems {
		return nil, validationError("context has too many entries")
	}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > maxContextItemLength {
			return nil, validationError("context item is too long")
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

func validateDirection(direction *models.Direction) error {
	if direction == nil {
		return validationError("direction is required")
	}

	dirType := models.DirectionType(strings.ToLower(strings.TrimSpace(string(direction.Type))))
	if _, ok := allowedDirectionTypes[dirType]; !ok {
		return validationError("direction.type is invalid")
	}
	direction.Type = dirType

	direction.Title = strings.TrimSpace(direction.Title)
	if direction.Title == "" {
		return validationError("direction.title is required")
	}
	if utf8.RuneCountInString(direction.Title) > maxDirectionTitleLength {
		return validationError("direction.title is too long")
	}

	direction.Description = strings.TrimSpace(direction.Description)
	if utf8.RuneCountInString(direction.Description) > maxDirectionDescLength {
		return validationError("direction.description is too long")
	}

	cleanedKeywords := make([]string, 0, len(direction.Keywords))
	for _, keyword := range direction.Keywords {
		trimmed := strings.TrimSpace(keyword)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > maxKeywordLength {
			return validationError("direction.keywords contains an entry that is too long")
		}
		cleanedKeywords = append(cleanedKeywords, trimmed)
		if len(cleanedKeywords) > maxDirectionKeywords {
			return validationError("direction.keywords has too many entries")
		}
	}
	direction.Keywords = cleanedKeywords

	if direction.Relevance < 0 || direction.Relevance > 1 {
		return validationError("direction.relevance must be between 0 and 1")
	}

	return nil
}

func validationError(msg string) error {
	return fmt.Errorf("%w: %s", appErrors.ErrInvalidRequest, msg)
}
