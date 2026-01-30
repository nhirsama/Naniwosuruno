package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nhirsama/Naniwosuruno/pkg"
	"github.com/nhirsama/Naniwosuruno/pkg/auth"
)

type ServerConnection struct {
	http          *http.Client
	baseURL       string
	clientID      string
	token         string
	authenticator auth.ClientAuthenticator
	useV1         bool
}

type UpdatePayload struct {
	Title string `json:"title"`
	OS    OSType `json:"os"`
}

func NewServerConnection(cfg *pkg.AppConfig) *ServerConnection {
	sc := &ServerConnection{
		http:     &http.Client{Timeout: 10 * time.Second},
		clientID: cfg.ClientID,
		token:    cfg.Token,
	}

	sc.configureBaseURL(cfg.BaseUrl)
	sc.initAuthenticator(cfg.PrivateKey)

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
	// 1. Get Challenge
	reqBody, _ := json.Marshal(auth.ChallengeRequest{ClientID: s.clientID})
	resp, err := s.http.Post(s.baseURL+"/api/v1/auth/challenge", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var challengeResp auth.ChallengeResponse
	if err := json.NewDecoder(resp.Body).Decode(&challengeResp); err != nil {
		return err
	}

	// 2. Sign
	sig, err := s.authenticator.SignChallenge(challengeResp.Challenge)
	if err != nil {
		return err
	}

	// 3. Verify
	verifyReq, _ := json.Marshal(auth.VerifyRequest{
		ClientID:  s.clientID,
		Signature: sig,
	})
	resp, err = s.http.Post(s.baseURL+"/api/v1/auth/verify", "application/json", bytes.NewBuffer(verifyReq))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("verify status %d", resp.StatusCode)
	}

	var verifyResp auth.VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return err
	}

	s.token = verifyResp.Token
	return nil
}

func (s *ServerConnection) SendUpdate(payload *UpdatePayload) error {
	payloadBytes, _ := json.Marshal(payload)

	path := "/api/v0/update"
	if s.useV1 {
		path = "/api/v1/update"
	}

	req, err := http.NewRequest("POST", s.baseURL+path, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "token", Value: s.token})

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("发送失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("服务端错误: %d", resp.StatusCode)
		if s.useV1 && resp.StatusCode == http.StatusUnauthorized {
			log.Println("Session 可能已过期，尝试重新认证...")
			if err := s.authenticateV1(); err != nil {
				return fmt.Errorf("重新认证失败: %v", err)
			}
			log.Println("重新认证成功，重试发送...")
			// Retry once
			return s.SendUpdate(payload)
		}
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	return nil
}
