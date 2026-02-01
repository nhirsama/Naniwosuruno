package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// --- 无状态认证器实现 (基础校验逻辑) ---

type serverAuthenticatorImpl struct {
	keyProvider KeyProvider
}

func NewServerAuthenticator(kp KeyProvider) ServerAuthenticator {
	return &serverAuthenticatorImpl{keyProvider: kp}
}

// GenerateChallenge 生成 32 字节随机数并编码，用于挑战-响应机制的防重放
func (s *serverAuthenticatorImpl) GenerateChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate challenge: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// VerifySignature 使用 Ed25519 算法验证签名是否由对应公钥的私钥对 nonce 进行签署
func (s *serverAuthenticatorImpl) VerifySignature(clientID string, nonce string, signatureBase64 string) (bool, error) {
	pubKeyBytes, err := s.keyProvider.GetClientPublicKey(clientID)
	if err != nil {
		return false, fmt.Errorf("client public key not found: %w", err)
	}

	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return false, errors.New("invalid public key size")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return false, fmt.Errorf("invalid signature format: %w", err)
	}

	return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), []byte(nonce), sigBytes), nil
}

// --- 有状态认证器实现 (管理会话与挑战状态) ---

type statefulAuthenticatorImpl struct {
	ServerAuthenticator
	keyProvider KeyProvider

	challenges     map[string]challengeData // 存储待验证的挑战信息
	challengesLock sync.Mutex

	sessions     map[string]sessionData // 存储已建立的会话信息
	sessionsLock sync.RWMutex
}

type challengeData struct {
	nonce     string
	expiresAt time.Time
}

type sessionData struct {
	clientID  string
	name      string
	expiresAt time.Time
}

func NewStatefulAuthenticator(kp KeyProvider) StatefulAuthenticator {
	sa := &statefulAuthenticatorImpl{
		ServerAuthenticator: NewServerAuthenticator(kp),
		keyProvider:         kp,
		challenges:          make(map[string]challengeData),
		sessions:            make(map[string]sessionData),
	}

	go sa.cleanupLoop() // 启动异步清理协程，防止内存因过期数据堆积而无限增长
	return sa
}

func (s *statefulAuthenticatorImpl) CreateChallenge(clientID string) (string, error) {
	nonce, err := s.GenerateChallenge()
	if err != nil {
		return "", err
	}

	s.challengesLock.Lock()
	defer s.challengesLock.Unlock()

	s.challenges[clientID] = challengeData{
		nonce:     nonce,
		expiresAt: time.Now().Add(30 * time.Second),
	}

	return nonce, nil
}

// ValidateChallengeAndIssueToken 是认证流程的核心，验证签名成功后会颁发一个临时的 Session Token
func (s *statefulAuthenticatorImpl) ValidateChallengeAndIssueToken(clientID, signature string) (string, int64, error) {
	s.challengesLock.Lock()
	data, ok := s.challenges[clientID]
	if ok {
		delete(s.challenges, clientID) // 用完即焚，彻底杜绝针对同一个 Challenge 的重放攻击
	}
	s.challengesLock.Unlock()

	if !ok {
		return "", 0, errors.New("challenge not found or already used")
	}
	if time.Now().After(data.expiresAt) {
		return "", 0, errors.New("challenge expired")
	}

	isValid, err := s.VerifySignature(clientID, data.nonce, signature)
	if err != nil {
		return "", 0, fmt.Errorf("verification error: %w", err)
	}
	if !isValid {
		return "", 0, errors.New("invalid signature")
	}

	token := uuid.New().String()
	expiresIn := int64(60 * 60 * 24) // Session 有效期设定为 1 天

	clientName := s.resolveClientName(clientID)

	s.sessionsLock.Lock()
	s.sessions[token] = sessionData{
		clientID:  clientID,
		name:      clientName,
		expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	s.sessionsLock.Unlock()

	return token, expiresIn, nil
}

func (s *statefulAuthenticatorImpl) ValidateSession(token string) (SessionInfo, bool) {
	s.sessionsLock.RLock()
	session, ok := s.sessions[token]
	s.sessionsLock.RUnlock()

	if !ok || time.Now().After(session.expiresAt) {
		return SessionInfo{}, false
	}

	return SessionInfo{ClientID: session.clientID, Name: session.name}, true
}

func (s *statefulAuthenticatorImpl) resolveClientName(clientID string) string {
	if kp, ok := s.keyProvider.(ClientNameProvider); ok {
		if name, err := kp.GetClientName(clientID); err == nil {
			return name
		}
	}
	return "Client-" + clientID
}

func (s *statefulAuthenticatorImpl) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		s.challengesLock.Lock()
		for id, data := range s.challenges {
			if now.After(data.expiresAt) {
				delete(s.challenges, id)
			}
		}
		s.challengesLock.Unlock()

		s.sessionsLock.Lock()
		for token, data := range s.sessions {
			if now.After(data.expiresAt) {
				delete(s.sessions, token)
			}
		}
		s.sessionsLock.Unlock()
	}
}

type ClientNameProvider interface {
	GetClientName(clientID string) (string, error)
}
