package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/teachain/exchange_server/internal/accessws/cache"
	"github.com/teachain/exchange_server/internal/accessws/model"
)

type AuthService struct {
	authURL    string
	signURL    string
	httpClient *http.Client
	nonceCache *nonceCache
}

type nonceCache struct {
	cache  *cache.Cache
	mu     sync.Mutex
	nonces map[string]bool
}

func newNonceCache(ttlSeconds float64) *nonceCache {
	return &nonceCache{
		cache:  cache.NewCache(ttlSeconds),
		nonces: make(map[string]bool),
	}
}

func (nc *nonceCache) isUsed(accessID, tonce string) bool {
	key := accessID + ":" + tonce
	if _, exists := nc.nonces[key]; exists {
		return true
	}
	_, found := nc.cache.Get(key)
	return found
}

func (nc *nonceCache) markUsed(accessID, tonce string) {
	key := accessID + ":" + tonce
	nc.nonces[key] = true
	nc.cache.Set(key, []byte("1"))
}

func (nc *nonceCache) clear(nonce string) {
	delete(nc.nonces, nonce)
}

type AuthResponse struct {
	Code int       `json:"code"`
	Data *AuthData `json:"data"`
}

type AuthData struct {
	UserID uint32 `json:"user_id"`
}

func NewAuthService(authURL, signURL string, nonceTTL float64) *AuthService {
	return &AuthService{
		authURL:    authURL,
		signURL:    signURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		nonceCache: newNonceCache(nonceTTL),
	}
}

func (s *AuthService) Authenticate(token, source string) (*AuthData, error) {
	body := map[string]string{"token": token, "source": source}
	b, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", s.authURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, err
	}

	if authResp.Code != 0 || authResp.Data == nil {
		return nil, fmt.Errorf("auth failed with code %d", authResp.Code)
	}
	return authResp.Data, nil
}

func (s *AuthService) VerifySignature(accessID, authorisation, tonce string) (*AuthData, error) {
	url := fmt.Sprintf("%s?access_id=%s&tonce=%s", s.signURL, accessID, tonce)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authorisation)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, err
	}

	if authResp.Code != 0 || authResp.Data == nil {
		return nil, fmt.Errorf("signature verification failed with code %d", authResp.Code)
	}
	return authResp.Data, nil
}

func (s *AuthService) AuthenticateSession(sess *model.ClientSession, token, source string) error {
	data, err := s.Authenticate(token, source)
	if err != nil {
		return err
	}
	sess.Auth = true
	sess.UserID = data.UserID
	sess.Source = source
	return nil
}

func (s *AuthService) VerifySessionSignature(sess *model.ClientSession, accessID, authorisation, tonce string) error {
	if s.nonceCache.isUsed(accessID, tonce) {
		return fmt.Errorf("tonce already used")
	}

	data, err := s.VerifySignature(accessID, authorisation, tonce)
	if err != nil {
		return err
	}

	s.nonceCache.markUsed(accessID, tonce)
	sess.Auth = true
	sess.UserID = data.UserID
	sess.Source = "api"
	return nil
}
