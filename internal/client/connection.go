package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"
	naniwosurunov1 "github.com/nhirsama/Naniwosuruno/gen/naniwosuruno/v1"
	"github.com/nhirsama/Naniwosuruno/gen/naniwosuruno/v1/naniwosurunov1connect"
	"github.com/nhirsama/Naniwosuruno/pkg"
	"github.com/nhirsama/Naniwosuruno/pkg/auth"
)

type ServerConnection struct {
	httpClient    *http.Client
	baseURL       string
	clientID      string
	token         string
	authenticator auth.ClientAuthenticator
	useV1         bool

	// RPC Clients
	authClient   naniwosurunov1connect.AuthServiceClient
	windowClient naniwosurunov1connect.WindowServiceClient
}

type UpdatePayload struct {
	Title string `json:"title"`
	OS    OSType `json:"os"`
}

func NewServerConnection(cfg *pkg.AppConfig) *ServerConnection {
	sc := &ServerConnection{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		clientID:   cfg.ClientID,
		token:      cfg.Token,
	}

	sc.configureBaseURL(cfg.BaseUrl)
	sc.initAuthenticator(cfg.PrivateKey)

	// Initialize RPC Clients
	sc.authClient = naniwosurunov1connect.NewAuthServiceClient(sc.httpClient, sc.baseURL)
	sc.windowClient = naniwosurunov1connect.NewWindowServiceClient(sc.httpClient, sc.baseURL)

	return sc
}

func (s *ServerConnection) configureBaseURL(url string) {
	s.baseURL = url
	if s.baseURL == "" {
		s.baseURL = "http://localhost:9975"
	}
}

func (s *ServerConnection) initAuthenticator(privateKey string) {
	if privateKey == "" {
		return
	}
	authenticator, err := auth.NewClientAuthenticatorFromBase64(privateKey)
	if err != nil {
		log.Printf("认证器初始化失败: %v", err)
		return
	}
	s.authenticator = authenticator
}

func (s *ServerConnection) Connect() {
	if s.authenticator == nil || s.clientID == "" {
		log.Println("跳过握手，使用 API v0 (Static Token)")
		s.useV1 = false
		return
	}

	if err := s.authenticateV1(); err != nil {
		log.Printf("认证失败: %v, 回退到 API v0", err)
		s.useV1 = false
	} else {
		log.Println("认证成功，使用 API v1")
		s.useV1 = true
	}
}

func (s *ServerConnection) authenticateV1() error {
	ctx := context.Background()

	// 1. Get Challenge
	req := connect.NewRequest(&naniwosurunov1.CreateChallengeRequest{
		ClientId: s.clientID,
	})

	res, err := s.authClient.CreateChallenge(ctx, req)
	if err != nil {
		return fmt.Errorf("create challenge failed: %w", err)
	}
	challenge := res.Msg.Challenge

	// 2. Sign
	sig, err := s.authenticator.SignChallenge(challenge)
	if err != nil {
		return fmt.Errorf("sign failed: %w", err)
	}

	// 3. Verify
	verifyReq := connect.NewRequest(&naniwosurunov1.VerifyChallengeRequest{
		ClientId:  s.clientID,
		Signature: sig,
	})

	verifyRes, err := s.authClient.VerifyChallenge(ctx, verifyReq)
	if err != nil {
		return fmt.Errorf("verify failed: %w", err)
	}

	s.token = verifyRes.Msg.Token
	return nil
}

func (s *ServerConnection) SendUpdate(payload *UpdatePayload) error {
	if !s.useV1 {
		return s.sendUpdateV0(payload)
	}

	return s.sendUpdateV1(payload)
}

func (s *ServerConnection) sendUpdateV1(payload *UpdatePayload) error {
	ctx := context.Background()
	req := connect.NewRequest(&naniwosurunov1.ReportWindowRequest{
		Title: payload.Title,
		Os:    string(payload.OS),
	})
	req.Header().Set("Authorization", "Bearer "+s.token)

	_, err := s.windowClient.ReportWindow(ctx, req)
	if err != nil {
		// Check for unauthenticated error to retry
		if connect.CodeOf(err) == connect.CodeUnauthenticated {
			log.Println("Session 可能已过期，尝试重新认证...")
			if reAuthErr := s.authenticateV1(); reAuthErr != nil {
				return fmt.Errorf("重新认证失败: %w", reAuthErr)
			}
			log.Println("重新认证成功，重试发送...")
			// Retry once
			req.Header().Set("Authorization", "Bearer "+s.token)
			_, err = s.windowClient.ReportWindow(ctx, req)
		}
	}

	return err
}

func (s *ServerConnection) sendUpdateV0(payload *UpdatePayload) error {
	payloadBytes, _ := json.Marshal(payload)
	path := "/api/v0/update"

	req, err := http.NewRequest("POST", s.baseURL+path, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "token", Value: s.token})

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	return nil
}
