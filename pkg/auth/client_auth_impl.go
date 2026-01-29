package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

// clientAuthenticatorImpl 是 ClientAuthenticator 的具体实现
type clientAuthenticatorImpl struct {
	privateKey ed25519.PrivateKey
}

// NewClientAuthenticator 创建一个新的 ClientAuthenticator 实例
// privateKey: 原始的 Ed25519 私钥字节 (64 bytes)
func NewClientAuthenticator(privateKey []byte) (ClientAuthenticator, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}
	return &clientAuthenticatorImpl{
		privateKey: ed25519.PrivateKey(privateKey),
	}, nil
}

// NewClientAuthenticatorFromBase64 从 Base64 编码的私钥字符串创建实例
// 这是一个辅助构造函数，方便从配置文件或环境变量加载
func NewClientAuthenticatorFromBase64(privateKeyBase64 string) (ClientAuthenticator, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 private key: %w", err)
	}
	return NewClientAuthenticator(keyBytes)
}

// SignChallenge 使用私钥对 Challenge 字符串进行签名
func (c *clientAuthenticatorImpl) SignChallenge(nonce string) (string, error) {
	// 签名是对 nonce 字符串的字节进行的
	signature := ed25519.Sign(c.privateKey, []byte(nonce))

	// 返回 Base64 编码的签名，以便通过 JSON 传输
	return base64.StdEncoding.EncodeToString(signature), nil
}
