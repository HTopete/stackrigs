package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/htopete/stackrigs/internal/config"
	"github.com/htopete/stackrigs/internal/middleware"
	"github.com/htopete/stackrigs/internal/model"
	"github.com/htopete/stackrigs/internal/store"
)

const sessionCookieName = "stackrigs_session"

// webAuthnUser adapts a model.Builder to the webauthn.User interface.
type webAuthnUser struct {
	builder     *model.Builder
	credentials []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", u.builder.ID))
}

func (u *webAuthnUser) WebAuthnName() string {
	return u.builder.Handle
}

func (u *webAuthnUser) WebAuthnDisplayName() string {
	return u.builder.DisplayName
}

func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func (u *webAuthnUser) WebAuthnIcon() string {
	return u.builder.AvatarURL
}

// webAuthnSession wraps a SessionData with its creation time for TTL cleanup.
type webAuthnSession struct {
	data      *webauthn.SessionData
	createdAt time.Time
}

// AuthHandler manages WebAuthn passkey and GitHub OAuth authentication flows.
type AuthHandler struct {
	authStore    *store.AuthStore
	builderStore *store.BuilderStore
	webAuthn     *webauthn.WebAuthn
	oauthConfig  *oauth2.Config
	cfg          *config.Config
	logger       *slog.Logger

	// In-memory session data for WebAuthn ceremonies (short-lived, 5-minute TTL).
	sessionMu     sync.RWMutex
	regSessions   map[string]*webAuthnSession
	loginSessions map[string]*webAuthnSession
}

// NewAuthHandler creates an AuthHandler with WebAuthn and GitHub OAuth configured.
func NewAuthHandler(
	authStore *store.AuthStore,
	builderStore *store.BuilderStore,
	cfg *config.Config,
	logger *slog.Logger,
) (*AuthHandler, error) {
	wconfig := &webauthn.Config{
		RPDisplayName: cfg.WebAuthnDisplayName,
		RPID:          cfg.WebAuthnRPID,
		RPOrigins:     cfg.WebAuthnRPOrigins,
	}

	wa, err := webauthn.New(wconfig)
	if err != nil {
		return nil, fmt.Errorf("initializing webauthn: %w", err)
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GitHubClientID,
		ClientSecret: cfg.GitHubClientSecret,
		Endpoint:     github.Endpoint,
		RedirectURL:  cfg.GitHubCallbackURL,
		Scopes:       []string{"read:user", "user:email"},
	}

	h := &AuthHandler{
		authStore:     authStore,
		builderStore:  builderStore,
		webAuthn:      wa,
		oauthConfig:   oauthCfg,
		cfg:           cfg,
		logger:        logger,
		regSessions:   make(map[string]*webAuthnSession),
		loginSessions: make(map[string]*webAuthnSession),
	}
	go h.cleanupExpiredSessions()
	return h, nil
}

// cleanupExpiredSessions removes stale WebAuthn ceremony sessions every minute.
func (h *AuthHandler) cleanupExpiredSessions() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-5 * time.Minute)
		h.sessionMu.Lock()
		for k, s := range h.regSessions {
			if s.createdAt.Before(cutoff) {
				delete(h.regSessions, k)
			}
		}
		for k, s := range h.loginSessions {
			if s.createdAt.Before(cutoff) {
				delete(h.loginSessions, k)
			}
		}
		h.sessionMu.Unlock()
	}
}

// ---- WebAuthn Registration ----

// BeginRegistration starts the WebAuthn registration ceremony for an authenticated builder.
func (h *AuthHandler) BeginRegistration(w http.ResponseWriter, r *http.Request) {
	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	creds, err := h.loadCredentials(builder.ID)
	if err != nil {
		h.logger.Error("failed to load credentials", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	user := &webAuthnUser{builder: builder, credentials: creds}

	options, session, err := h.webAuthn.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementPreferred,
			UserVerification: protocol.VerificationPreferred,
		}),
	)
	if err != nil {
		h.logger.Error("failed to begin webauthn registration", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to begin registration")
		return
	}

	h.sessionMu.Lock()
	h.regSessions[builder.Handle] = &webAuthnSession{data: session, createdAt: time.Now()}
	h.sessionMu.Unlock()

	writeJSON(w, http.StatusOK, options)
}

// FinishRegistration completes the WebAuthn registration ceremony.
func (h *AuthHandler) FinishRegistration(w http.ResponseWriter, r *http.Request) {
	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	h.sessionMu.Lock()
	entry, ok := h.regSessions[builder.Handle]
	if ok {
		delete(h.regSessions, builder.Handle)
	}
	h.sessionMu.Unlock()

	if !ok {
		writeError(w, http.StatusBadRequest, "no registration in progress")
		return
	}
	session := entry.data

	creds, err := h.loadCredentials(builder.ID)
	if err != nil {
		h.logger.Error("failed to load credentials", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	user := &webAuthnUser{builder: builder, credentials: creds}

	credential, err := h.webAuthn.FinishRegistration(user, *session, r)
	if err != nil {
		h.logger.Error("failed to finish webauthn registration", "error", err)
		writeError(w, http.StatusBadRequest, "registration verification failed")
		return
	}

	transports := make([]string, 0, len(credential.Transport))
	for _, t := range credential.Transport {
		transports = append(transports, string(t))
	}

	credID := hex.EncodeToString(credential.ID)
	credRow := store.WebAuthnCredentialRow{
		ID:              credID,
		BuilderID:       builder.ID,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		Transport:       strings.Join(transports, ","),
		SignCount:       credential.Authenticator.SignCount,
	}

	if err := h.authStore.SaveCredential(credRow); err != nil {
		h.logger.Error("failed to save credential", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save credential")
		return
	}

	h.logger.Info("webauthn credential registered", "builder", builder.Handle)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok", "credential_id": credID})
}

// ---- WebAuthn Login ----

// BeginLogin starts the WebAuthn login ceremony.
func (h *AuthHandler) BeginLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Handle string `json:"handle"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	if body.Handle == "" {
		// Discoverable credential flow (resident key)
		options, session, err := h.webAuthn.BeginDiscoverableLogin()
		if err != nil {
			h.logger.Error("failed to begin discoverable login", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to begin login")
			return
		}

		challengeKey := base64.RawURLEncoding.EncodeToString([]byte(session.Challenge))
		h.sessionMu.Lock()
		h.loginSessions[challengeKey] = &webAuthnSession{data: session, createdAt: time.Now()}
		h.sessionMu.Unlock()

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"publicKey":     options,
			"challenge_key": challengeKey,
		})
		return
	}

	builder, err := h.builderStore.GetByHandle(body.Handle)
	if err != nil {
		h.logger.Error("failed to get builder for login", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if builder == nil {
		writeError(w, http.StatusNotFound, "builder not found")
		return
	}

	creds, err := h.loadCredentials(builder.ID)
	if err != nil {
		h.logger.Error("failed to load credentials", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if len(creds) == 0 {
		writeError(w, http.StatusBadRequest, "no passkeys registered for this account")
		return
	}

	user := &webAuthnUser{builder: builder, credentials: creds}

	options, session, err := h.webAuthn.BeginLogin(user)
	if err != nil {
		h.logger.Error("failed to begin webauthn login", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to begin login")
		return
	}

	h.sessionMu.Lock()
	h.loginSessions[builder.Handle] = &webAuthnSession{data: session, createdAt: time.Now()}
	h.sessionMu.Unlock()

	writeJSON(w, http.StatusOK, options)
}

// FinishLogin completes the WebAuthn login ceremony and creates a session.
func (h *AuthHandler) FinishLogin(w http.ResponseWriter, r *http.Request) {
	handle := r.URL.Query().Get("handle")
	challengeKey := r.URL.Query().Get("challenge_key")

	var builder *model.Builder

	if challengeKey != "" {
		// Discoverable credential flow
		h.sessionMu.Lock()
		entry, ok := h.loginSessions[challengeKey]
		if ok {
			delete(h.loginSessions, challengeKey)
		}
		h.sessionMu.Unlock()
		if !ok {
			writeError(w, http.StatusBadRequest, "no login in progress")
			return
		}
		session := entry.data

		parsedResponse, err := protocol.ParseCredentialRequestResponse(r)
		if err != nil {
			h.logger.Error("failed to parse credential response", "error", err)
			writeError(w, http.StatusBadRequest, "invalid credential response")
			return
		}

		userHandle := string(parsedResponse.Response.UserHandle)
		var builderID int64
		if _, err := fmt.Sscanf(userHandle, "%d", &builderID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid user handle")
			return
		}

		builder, err = h.builderStore.GetByID(builderID)
		if err != nil || builder == nil {
			writeError(w, http.StatusNotFound, "builder not found")
			return
		}

		creds, err := h.loadCredentials(builder.ID)
		if err != nil {
			h.logger.Error("failed to load credentials", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		user := &webAuthnUser{builder: builder, credentials: creds}

		credential, err := h.webAuthn.ValidateDiscoverableLogin(
			func(rawID, userHandle []byte) (webauthn.User, error) {
				return user, nil
			},
			*session,
			parsedResponse,
		)
		if err != nil {
			h.logger.Error("discoverable login validation failed", "error", err)
			writeError(w, http.StatusUnauthorized, "login verification failed")
			return
		}

		credHexID := hex.EncodeToString(credential.ID)
		_ = h.authStore.UpdateCredentialSignCount(credHexID, credential.Authenticator.SignCount)

	} else if handle != "" {
		h.sessionMu.Lock()
		entry, ok := h.loginSessions[handle]
		if ok {
			delete(h.loginSessions, handle)
		}
		h.sessionMu.Unlock()
		if !ok {
			writeError(w, http.StatusBadRequest, "no login in progress")
			return
		}
		session := entry.data

		var err error
		builder, err = h.builderStore.GetByHandle(handle)
		if err != nil || builder == nil {
			writeError(w, http.StatusNotFound, "builder not found")
			return
		}

		creds, err := h.loadCredentials(builder.ID)
		if err != nil {
			h.logger.Error("failed to load credentials", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		user := &webAuthnUser{builder: builder, credentials: creds}

		credential, err := h.webAuthn.FinishLogin(user, *session, r)
		if err != nil {
			h.logger.Error("webauthn login verification failed", "error", err)
			writeError(w, http.StatusUnauthorized, "login verification failed")
			return
		}

		credHexID := hex.EncodeToString(credential.ID)
		_ = h.authStore.UpdateCredentialSignCount(credHexID, credential.Authenticator.SignCount)
	} else {
		writeError(w, http.StatusBadRequest, "handle or challenge_key query parameter required")
		return
	}

	// Create session
	maxAge := time.Duration(h.cfg.SessionMaxAge) * time.Second
	sessionID, err := h.authStore.CreateSession(builder.ID, maxAge)
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	h.setSessionCookie(w, sessionID)

	h.logger.Info("webauthn login successful", "builder", builder.Handle)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"builder": builder,
	})
}

// ---- GitHub OAuth ----

// GitHubRedirect redirects the user to GitHub's OAuth authorization page.
func (h *AuthHandler) GitHubRedirect(w http.ResponseWriter, r *http.Request) {
	state, err := generateOAuthState()
	if err != nil {
		h.logger.Error("failed to generate oauth state", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.IsProd(),
	})

	url := h.oauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GitHubCallback handles the OAuth callback from GitHub.
func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing oauth state cookie")
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		writeError(w, http.StatusForbidden, "invalid oauth state")
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	token, err := h.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("github oauth exchange failed", "error", err)
		writeError(w, http.StatusInternalServerError, "oauth exchange failed")
		return
	}

	// Fetch GitHub user info
	client := h.oauthConfig.Client(r.Context(), token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		h.logger.Error("failed to fetch github user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch github user")
		return
	}
	defer resp.Body.Close()

	var ghUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		h.logger.Error("failed to decode github user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to decode github user")
		return
	}

	displayName := ghUser.Name
	if displayName == "" {
		displayName = ghUser.Login
	}

	builderID, created, err := h.authStore.FindOrCreateBuilderByGitHub(
		ghUser.ID, ghUser.Login, displayName, ghUser.AvatarURL, token.AccessToken,
	)
	if err != nil {
		h.logger.Error("failed to find or create builder from github", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to process github login")
		return
	}

	if created {
		h.logger.Info("new builder created from github", "github_user", ghUser.Login, "builder_id", builderID)
	}

	maxAge := time.Duration(h.cfg.SessionMaxAge) * time.Second
	sessionID, err := h.authStore.CreateSession(builderID, maxAge)
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	h.setSessionCookie(w, sessionID)

	// New builders go to /new-build for onboarding; returning builders to home.
	redirectPath := "/?auth=success"
	if created {
		redirectPath = "/new-build?welcome=1"
	}
	http.Redirect(w, r, h.cfg.FrontendURL+redirectPath, http.StatusSeeOther)
}

// ---- Session Management ----

// Logout destroys the current session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		if err := h.authStore.DeleteSession(cookie.Value); err != nil {
			h.logger.Error("failed to delete session", "error", err)
		}
	}

	clearCookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.IsProd(),
	}
	if h.cfg.CookieDomain != "" {
		clearCookie.Domain = h.cfg.CookieDomain
	}
	http.SetCookie(w, clearCookie)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Me returns the currently authenticated builder.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	ghConn, _ := h.authStore.GetGitHubConnection(builder.ID)

	response := map[string]interface{}{
		"builder": builder,
	}
	if ghConn != nil {
		response["github"] = map[string]interface{}{
			"username":   ghConn.GitHubUsername,
			"avatar_url": ghConn.GitHubAvatarURL,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// ---- Helpers ----

func (h *AuthHandler) loadCredentials(builderID int64) ([]webauthn.Credential, error) {
	rows, err := h.authStore.GetCredentialsByBuilder(builderID)
	if err != nil {
		return nil, err
	}

	creds := make([]webauthn.Credential, 0, len(rows))
	for _, row := range rows {
		transports := make([]protocol.AuthenticatorTransport, 0)
		if row.Transport != "" {
			for _, t := range strings.Split(row.Transport, ",") {
				transports = append(transports, protocol.AuthenticatorTransport(strings.TrimSpace(t)))
			}
		}

		creds = append(creds, webauthn.Credential{
			ID:              row.CredentialID,
			PublicKey:       row.PublicKey,
			AttestationType: row.AttestationType,
			Transport:       transports,
			Authenticator: webauthn.Authenticator{
				SignCount: row.SignCount,
			},
		})
	}

	return creds, nil
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, sessionID string) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   h.cfg.SessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.IsProd(),
	}
	if h.cfg.CookieDomain != "" {
		cookie.Domain = h.cfg.CookieDomain
	}
	http.SetCookie(w, cookie)
}

func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
