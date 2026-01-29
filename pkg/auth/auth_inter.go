package auth

// KeyProvider 定义了服务端获取客户端公钥的接口
type KeyProvider interface {
	GetClientPublicKey(clientID string) ([]byte, error)
}

// ServerAuthenticator 定义了基础的无状态认证逻辑接口
type ServerAuthenticator interface {
	GenerateChallenge() (string, error)
	VerifySignature(clientID string, nonce string, signatureBase64 string) (bool, error)
}

// StatefulAuthenticator 扩展了 ServerAuthenticator，增加了有状态的 Challenge 和 Session 管理
type StatefulAuthenticator interface {
	ServerAuthenticator

	CreateChallenge(clientID string) (string, error)
	ValidateChallengeAndIssueToken(clientID, signature string) (string, int64, error)
	ValidateSession(token string) (SessionInfo, bool)
}

// SessionInfo 存储会话信息
type SessionInfo struct {
	ClientID string
	Name     string
}

// ClientAuthenticator 定义了客户端认证逻辑的接口
type ClientAuthenticator interface {
	SignChallenge(nonce string) (string, error)
}

// DTOs

type ChallengeRequest struct {
	ClientID string `json:"client_id"`
}

type ChallengeResponse struct {
	Challenge string `json:"challenge"`
}

type VerifyRequest struct {
	ClientID  string `json:"client_id"`
	Signature string `json:"signature"`
}

type VerifyResponse struct {
	Token     string `json:"token,omitempty"`
	ExpiresIn int64  `json:"expires_in,omitempty"`
}
