package googleAuthService

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	spreadsheetRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/spreadsheet"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

const (
	scopeOpenID = "openid"
	scopeEmail  = "email"
	stateBytes  = 32
)

func (s *service) oauthConfig(callbackURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		Endpoint:     s.endpoint,
		RedirectURL:  callbackURL,
		Scopes:       []string{drive.DriveFileScope, scopeOpenID, scopeEmail},
	}
}

func (s *service) connect(ctx context.Context, refreshToken, account string, connectedAt time.Time) (*connection, error) {
	sheets, err := s.newSpreadsheets(ctx, spreadsheetRepository.Config{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		RefreshToken: refreshToken,
		FolderID:     s.folderID,
		CompanyName:  s.companyName,
		Timeout:      s.timeout,
	})
	if err != nil {
		return nil, err
	}
	return &connection{sheets: sheets, token: refreshToken, account: account, connectedAt: connectedAt}, nil
}

func (s *service) swap(conn *connection) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = conn.sheets.Close()
		return ErrClosed
	}
	old := s.current
	s.current = conn
	s.mu.Unlock()
	if old != nil {
		_ = old.sheets.Close()
	}
	return nil
}

func (s *service) prunePending(now time.Time) {
	var oldestState string
	var oldest time.Time
	for state, flow := range s.pending {
		if !now.Before(flow.expires) {
			delete(s.pending, state)
			continue
		}
		if oldestState == "" || flow.expires.Before(oldest) {
			oldestState, oldest = state, flow.expires
		}
	}
	if len(s.pending) >= maxPending && oldestState != "" {
		delete(s.pending, oldestState)
	}
}

func (s *service) takePending(state string, now time.Time) (pending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	flow, ok := s.pending[state]
	if !ok || state == "" {
		return pending{}, false
	}
	delete(s.pending, state)
	if !now.Before(flow.expires) {
		return pending{}, false
	}
	return flow, true
}

func (s *service) revoke(ctx context.Context, token string) error {
	revokeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(revokeCtx, http.MethodPost, s.revokeURL, strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("revoke status %d", response.StatusCode)
	}
	return nil
}

func randomState() (string, error) {
	buf := make([]byte, stateBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func absoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func accountFromIDToken(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	claims, err := utils.FromStringToObjectJSON[struct {
		Email string `json:"email"`
	}](string(payload))
	if err != nil {
		return ""
	}
	return claims.Email
}

func recoverAsError(err *error) {
	if recovered := recover(); recovered != nil {
		*err = fmt.Errorf("%w: %v", ErrPanicked, recovered)
	}
}
