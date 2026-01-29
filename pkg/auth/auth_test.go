package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
)

// mockKeyProvider 模拟 KeyProvider 接口实现
type mockKeyProvider struct {
	keys map[string][]byte
}

func (m *mockKeyProvider) GetClientPublicKey(clientID string) ([]byte, error) {
	key, ok := m.keys[clientID]
	if !ok {
		return nil, errors.New("client not found")
	}
	return key, nil
}

// 模拟 ClientNameProvider
func (m *mockKeyProvider) GetClientName(clientID string) (string, error) {
	if _, ok := m.keys[clientID]; ok {
		return "MockClient-" + clientID, nil
	}
	return "", errors.New("client not found")
}

func TestStatelessAuth(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	clientID := "test-client-stateless"

	mkp := &mockKeyProvider{keys: map[string][]byte{clientID: pubKey}}
	server := NewServerAuthenticator(mkp)
	client, _ := NewClientAuthenticator(privKey)

	challenge, err := server.GenerateChallenge()
	if err != nil {
		t.Fatal(err)
	}

	sig, err := client.SignChallenge(challenge)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := server.VerifySignature(clientID, challenge, sig)
	if err != nil || !valid {
		t.Error("Stateless verification failed")
	}
}

func TestStatefulAuth(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	clientID := "test-client-stateful"

	mkp := &mockKeyProvider{keys: map[string][]byte{clientID: pubKey}}

	// 使用 Stateful Authenticator
	server := NewStatefulAuthenticator(mkp)
	client, _ := NewClientAuthenticator(privKey)

	// 1. Create Challenge
	nonce, err := server.CreateChallenge(clientID)
	if err != nil {
		t.Fatalf("CreateChallenge failed: %v", err)
	}
	if nonce == "" {
		t.Fatal("Empty nonce")
	}

	// 2. Client Signs
	sig, err := client.SignChallenge(nonce)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Issue Token
	token, ttl, err := server.ValidateChallengeAndIssueToken(clientID, sig)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}
	if token == "" || ttl <= 0 {
		t.Error("Invalid token or ttl")
	}

	// 4. Validate Session
	session, ok := server.ValidateSession(token)
	if !ok {
		t.Error("Session validation failed")
	}
	if session.ClientID != clientID {
		t.Errorf("Session ClientID mismatch: got %s, want %s", session.ClientID, clientID)
	}
	if session.Name != "MockClient-"+clientID {
		t.Errorf("Session Name mismatch: got %s", session.Name)
	}

	// 5. Test Replay Attack (Challenge should be consumed)
	_, _, err = server.ValidateChallengeAndIssueToken(clientID, sig)
	if err == nil {
		t.Error("Replay attack should fail (challenge consumed)")
	}

	// 6. Test Invalid Token
	_, ok = server.ValidateSession("invalid-token-xyz")
	if ok {
		t.Error("Invalid token should fail")
	}
}

func TestNewClientAuthenticatorFromBase64(t *testing.T) {
	_, privKey, _ := ed25519.GenerateKey(nil)
	privBase64 := base64.StdEncoding.EncodeToString(privKey)

	_, err := NewClientAuthenticatorFromBase64(privBase64)
	if err != nil {
		t.Errorf("failed to create authenticator from base64: %v", err)
	}
}
