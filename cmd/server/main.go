package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	Port         int    `yaml:"port" json:"port"`
	MCPPort      int    `yaml:"mcp_port" json:"mcp_port"`
	LLMAPIKey    string `yaml:"llm_api_key" json:"llm_api_key"`
	LLMBaseURL   string `yaml:"llm_base_url" json:"llm_base_url"`
	LLMModel     string `yaml:"llm_model" json:"llm_model"`
	DataDir      string `yaml:"data_dir" json:"data_dir"`
	WebDir       string `yaml:"web_dir" json:"web_dir"`
	UseFileStore bool   `yaml:"use_file_store" json:"use_file_store"`
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

	mcpServer := setupMCPServer(thoughtExpander, sessionManager)
	if err := mcpServer.Start(cfg.MCPPort); err != nil {
		utils.Errorf("failed to start MCP server: %v", err)
		os.Exit(1)
	}

	webMux := setupWebServer(cfg.WebDir, sessionManager, thoughtExpander)
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
		Port:         8080,
		MCPPort:      9090,
		LLMModel:     "gpt-4.1",
		WebDir:       "web",
		UseFileStore: false,
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

func setupMCPServer(te *services.ThoughtExpander, sm *services.SessionManager) *mcp.MCPServer {
	server := mcp.NewMCPServer(te, sm)
	server.RegisterTool("expand_thought", mcp.NewExpandThoughtTool(te))
	server.RegisterTool("explore_direction", mcp.NewExploreDirectionTool(te))
	server.RegisterTool("create_session", mcp.NewCreateSessionTool(sm))
	server.RegisterTool("get_session", mcp.NewGetSessionTool(sm))
	return server
}

func setupWebServer(webDir string, sessionManager *services.SessionManager, expander *services.ThoughtExpander) *http.ServeMux {
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

	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var payload struct {
				UserID  string `json:"user_id"`
				Concept string `json:"concept"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				respondError(w, fmt.Errorf("%w: %v", appErrors.ErrInvalidRequest, err))
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
	})

	mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
		if id == "" {
			respondError(w, appErrors.ErrInvalidRequest)
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
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				respondError(w, fmt.Errorf("%w: %v", appErrors.ErrInvalidRequest, err))
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
	})

	mux.HandleFunc("/api/expand", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Concept       string   `json:"concept"`
			Context       []string `json:"context"`
			ExpansionType string   `json:"expansion_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			respondError(w, fmt.Errorf("%w: %v", appErrors.ErrInvalidRequest, err))
			return
		}
		result, err := expander.Expand(&services.ExpansionRequest{
			Concept:       payload.Concept,
			Context:       payload.Context,
			ExpansionType: models.DirectionType(payload.ExpansionType),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, result)
	})

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
