package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nhirsama/Naniwosuruno/gen/naniwosuruno/v1/naniwosurunov1connect"
	"github.com/nhirsama/Naniwosuruno/internal/server/v0"
	"github.com/nhirsama/Naniwosuruno/internal/service"
	"github.com/nhirsama/Naniwosuruno/pkg"
	"github.com/nhirsama/Naniwosuruno/pkg/auth"
	"github.com/r3labs/sse/v2"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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
}

func (s *Server) initSSEServer() {
	s.sseServer = sse.New()
	s.sseServer.EventTTL = 24 * time.Hour
	s.sseServer.BufferSize = 4
	s.sseServer.CreateStream("focus")
}

func (s *Server) registerRoutes() {
	mux := http.NewServeMux()

	// 1. Register ConnectRPC Services
	authSvc := service.NewAuthService(s.authenticator)
	windowSvc := service.NewWindowService(s.sseServer, s.authenticator)

	authPath, authHandler := naniwosurunov1connect.NewAuthServiceHandler(authSvc)
	mux.Handle(authPath, authHandler)

	winPath, winHandler := naniwosurunov1connect.NewWindowServiceHandler(windowSvc)
	mux.Handle(winPath, winHandler)

	// 2. Legacy V0 API
	v0Handler := v0.NewHandler(s.configManager, s.sseServer)
	mux.HandleFunc("/api/v0/update", v0Handler.HandleUpdate)
	mux.HandleFunc("/events", v0Handler.HandleEvents)

	// Support existing frontend SSE path
	mux.HandleFunc("/api/v1/events", v0Handler.HandleEvents)

	// 3. Static Files
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	fmt.Println("服务端启动于 :9975")
	// Use h2c to support HTTP/2 without TLS (Cleartext)
	if err := http.ListenAndServe(":9975", h2c.NewHandler(mux, &http2.Server{})); err != nil {
		log.Fatal("服务启动失败", err)
	}
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
