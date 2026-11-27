package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	dotenv "github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

const waitTimeout = 5 * time.Minute

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	_ = dotenv.Load(".env")
	clientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	if clientID == "" || clientSecret == "" {
		return errors.New("set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET in .env first (OAuth client of type Desktop app)")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	redirect := fmt.Sprintf("http://%s/callback", listener.Addr().String())
	cfg := &oauth2.Config{ClientID: clientID, ClientSecret: clientSecret, Endpoint: google.Endpoint, RedirectURL: redirect, Scopes: []string{drive.DriveFileScope}}
	state, err := randomState()
	if err != nil {
		return err
	}
	verifier := oauth2.GenerateVerifier()
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"), oauth2.S256ChallengeOption(verifier))

	codes := make(chan string, 1)
	failures := make(chan error, 1)
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			failures <- errors.New("state mismatch, try again")
			return
		}
		if msg := r.URL.Query().Get("error"); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			failures <- fmt.Errorf("google returned %s", msg)
			return
		}
		fmt.Fprintln(w, "Signed in. You can close this tab and go back to the terminal.")
		codes <- r.URL.Query().Get("code")
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	fmt.Println("Open this link in your browser and sign in with the Google account that should own the snapshot spreadsheets:")
	fmt.Println()
	fmt.Println(url)
	fmt.Println()

	var code string
	select {
	case code = <-codes:
	case err := <-failures:
		return err
	case <-time.After(waitTimeout):
		return errors.New("timed out waiting for the browser sign-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return err
	}
	if token.RefreshToken == "" {
		return errors.New("google did not return a refresh token; remove the app's access at myaccount.google.com/permissions and run this again")
	}
	fmt.Println("Add this line to .env (and to Render's environment):")
	fmt.Println()
	fmt.Printf("GOOGLE_REFRESH_TOKEN='%s'\n", token.RefreshToken)
	return nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
