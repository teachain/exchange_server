package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/viabtc/go-project/services/accessws/internal/model"
	"net/http"
	"time"
)

type AuthService struct {
	authURL    string
	signURL    string
	httpClient *http.Client
}

type AuthResponse struct {
	Code int       `json:"code"`
	Data *AuthData `json:"data"`
}

type AuthData struct {
	UserID uint32 `json:"user_id"`
}

func NewAuthService(authURL, signURL string) *AuthService {
	return &AuthService{
		authURL:    authURL,
		signURL:    signURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
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
	data, err := s.VerifySignature(accessID, authorisation, tonce)
	if err != nil {
		return err
	}
	sess.Auth = true
	sess.UserID = data.UserID
	sess.Source = "api"
	return nil
}
