package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/nhirsama/Naniwosuruno/pkg"
	"github.com/nhirsama/Naniwosuruno/pkg/auth"
	"github.com/r3labs/sse/v2"
)

type Server struct {
	configManager *pkg.ConfigManager
	authenticator auth.StatefulAuthenticator
	sseServer     *sse.Server
}

func Run() {
	NewServer().Run()
}

func NewServer() *Server {
	cm, err := pkg.NewConfigManager()
	if err != nil {
		log.Fatalf("初始化配置管理器失败: %v", err)
	}

	keyProvider := &ConfigKeyProvider{cm: cm}

	return &Server{
		configManager: cm,
		authenticator: auth.NewStatefulAuthenticator(keyProvider),
	}
}

func (s *Server) Run() {
	s.initSSEServer()
	s.registerRoutes()

	fmt.Println("服务端启动于 :9975")
	if err := http.ListenAndServe(":9975", nil); err != nil {
		log.Fatal("服务启动失败", err)
	}
}

func (s *Server) initSSEServer() {
	s.sseServer = sse.New()
	s.sseServer.EventTTL = 24 * time.Hour
	s.sseServer.BufferSize = 4
	s.sseServer.CreateStream("focus")
}

func (s *Server) registerRoutes() {
	// API V1: 基于 Ed25519 挑战-响应机制的安全认证接口
	http.HandleFunc("/api/v1/auth/challenge", s.handleChallengeV1)
	http.HandleFunc("/api/v1/auth/verify", s.handleVerifyV1)
	http.HandleFunc("/api/v1/update", s.handleUpdateV1)
	http.HandleFunc("/api/v1/events", s.handleEventsV1)

	// API V0: 传统的基于静态 Token 的认证接口，用于兼容旧版本客户端
	http.HandleFunc("/api/v0/update", s.handleUpdateV0)
	http.HandleFunc("/api/v0/events", s.handleEventsV0)
	http.HandleFunc("/update", s.handleUpdateV0) // 保持根路径兼容性
	http.HandleFunc("/events", s.handleEventsV0)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
}

// --- V1 Handlers ---

func (s *Server) handleChallengeV1(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req auth.ChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	nonce, err := s.authenticator.CreateChallenge(req.ClientID)
	if err != nil {
		log.Printf("Create challenge error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(auth.ChallengeResponse{Challenge: nonce})
}

func (s *Server) handleVerifyV1(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req auth.VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token, expiresIn, err := s.authenticator.ValidateChallengeAndIssueToken(req.ClientID, req.Signature)
	if err != nil {
		log.Printf("Auth failed for %s: %v", req.ClientID, err)
		http.Error(w, "Authentication failed", http.StatusForbidden)
		return
	}

	json.NewEncoder(w).Encode(auth.VerifyResponse{Token: token, ExpiresIn: expiresIn})
}

func (s *Server) handleUpdateV1(w http.ResponseWriter, r *http.Request) {
	session, ok := s.validateTokenV1(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	s.processUpdate(w, r, session.Name)
}

func (s *Server) handleEventsV1(w http.ResponseWriter, r *http.Request) {
	s.serveSSE(w, r)
}

// --- V0 Handlers (Legacy) ---

func (s *Server) handleUpdateV0(w http.ResponseWriter, r *http.Request) {
	if !s.validateTokenV0(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	s.processUpdate(w, r, "Legacy Client")
}

func (s *Server) handleEventsV0(w http.ResponseWriter, r *http.Request) {
	s.serveSSE(w, r)
}

// --- Helpers ---

func (s *Server) validateTokenV1(r *http.Request) (auth.SessionInfo, bool) {
	token := getTokenFromRequest(r)
	if token == "" {
		return auth.SessionInfo{}, false
	}
	return s.authenticator.ValidateSession(token)
}

func (s *Server) validateTokenV0(r *http.Request) bool {
	token := getTokenFromRequest(r)
	if token == "" {
		return false
	}

	cfg := s.configManager.GetConfig()
	return cfg.Token != "" && token == cfg.Token
}

func getTokenFromRequest(r *http.Request) string {
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	if c, err := r.Cookie("token"); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) serveSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	s.sseServer.ServeHTTP(w, r)
}

// processUpdate 处理具体的窗口信息更新逻辑，并分发到 SSE 流中
func (s *Server) processUpdate(w http.ResponseWriter, r *http.Request, clientName string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		log.Printf("Reading body failed: %v", err)
		return
	}

	var updateData struct {
		Title string `json:"title"`
		OS    string `json:"os"`
	}
	_ = json.Unmarshal(body, &updateData)

	// 构造统一格式的消息负载，包含客户端显示名称
	payload := fmt.Sprintf(`{"title": %q, "os": %q, "client": %q}`, updateData.Title, updateData.OS, clientName)
	s.sseServer.TryPublish("focus", &sse.Event{Data: []byte(payload)})

	fmt.Fprintf(w, "Update received from %s", clientName)
}

// --- KeyProvider Implementation ---

type ConfigKeyProvider struct {
	cm *pkg.ConfigManager
}

// GetClientPublicKey 实现了自动重载逻辑：如果内存缓存中找不到 ID，会尝试刷新磁盘配置文件
func (p *ConfigKeyProvider) GetClientPublicKey(clientID string) ([]byte, error) {
	client, ok := p.cm.GetClient(clientID)
	if !ok {
		// 这里是解决本地调试中客户端生成新密钥并保存后，服务端能及时感知到的核心逻辑
		if err := p.cm.Reload(); err != nil {
			log.Printf("Config reload failed: %v", err)
			return nil, errors.New("client not found and reload failed")
		}
		client, ok = p.cm.GetClient(clientID)
	}

	if !ok {
		return nil, errors.New("client not found")
	}
	return base64.StdEncoding.DecodeString(client.PublicKey)
}

func (p *ConfigKeyProvider) GetClientName(clientID string) (string, error) {
	client, ok := p.cm.GetClient(clientID)
	if !ok {
		return "", errors.New("client not found")
	}
	return client.Name, nil
}
