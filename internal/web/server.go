// Package web serves the Twitch authorization flow.
//
// It exists because Twitch only issues a user token by redirecting a browser
// back to a URI registered on the application. Without it, tokens have to be
// pasted into the database by hand, which is what made the bot impossible to
// set up and impossible to offer to a second streamer.
package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"mitoboat/internal/config"
	"mitoboat/internal/twitch"
)

// Registrar receives completed authorizations.
//
// It is an interface so the server can run on its own during bootstrap, when
// there is no bot token yet and therefore no running bot to register into.
type Registrar interface {
	// RegisterBot stores the token the bot posts as.
	RegisterBot(ctx context.Context, user *twitch.AuthorizedUser) error
	// RegisterStreamer registers a channel and joins it if the bot is running.
	RegisterStreamer(ctx context.Context, user *twitch.AuthorizedUser) error
}

// Server is the authorization HTTP server.
type Server struct {
	cfg        *config.Config
	oauth      *twitch.OAuth
	registrar  Registrar
	httpServer *http.Server
}

// New builds the authorization server.
func New(cfg *config.Config, oauth *twitch.OAuth, registrar Registrar) *Server {
	s := &Server{cfg: cfg, oauth: oauth, registrar: registrar}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /auth/bot", s.handleAuthorizeBot)
	mux.HandleFunc("GET /auth/streamer", s.handleAuthorizeStreamer)
	mux.HandleFunc("GET /auth/callback", s.handleCallback)

	s.httpServer = &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: securityHeaders(mux),
		// Explicit timeouts: without ReadHeaderTimeout a client can hold a
		// connection open indefinitely by trickling headers.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return s
}

// Run serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("Authorization server listening",
			"scope", "WEB", "addr", s.cfg.HTTPAddr, "redirect_uri", s.cfg.RedirectURI)
		err := s.httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("authorization server: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Warn("Authorization server did not shut down cleanly", "scope", "WEB", "error", err)
		}
		<-errCh
		return nil
	}
}

// authorized reports whether a request carries the admin secret.
//
// The comparison is constant time so the secret cannot be recovered by timing
// repeated guesses.
func (s *Server) authorized(r *http.Request) bool {
	key := r.URL.Query().Get("key")
	return subtle.ConstantTimeCompare([]byte(key), []byte(s.cfg.AdminSecret)) == 1
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	render(w, http.StatusOK, pageData{
		Title: "MitoBoat",
		Body:  "Streamers can add the bot to their channel below.",
		Links: []link{{Href: "/auth/streamer", Label: "Add MitoBoat to my channel"}},
	})
}

// handleAuthorizeBot starts the flow for the account the bot posts as. It is
// gated: this token is what lets the bot speak, in every channel at once.
func (s *Server) handleAuthorizeBot(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		slog.Warn("Rejected an unauthorized bot authorization attempt",
			"scope", "WEB", "remote", r.RemoteAddr)
		render(w, http.StatusForbidden, pageData{
			Title: "Forbidden",
			Body:  "This page requires the administrator key.",
		})
		return
	}
	s.redirectToTwitch(w, r, twitch.RoleBot)
}

// handleAuthorizeStreamer starts the flow for a channel owner. It is open on
// purpose: authorizing grants access to the streamer's own channel and nothing
// else, and self-service is the point of running one bot for many streamers.
func (s *Server) handleAuthorizeStreamer(w http.ResponseWriter, r *http.Request) {
	s.redirectToTwitch(w, r, twitch.RoleStreamer)
}

func (s *Server) redirectToTwitch(w http.ResponseWriter, r *http.Request, role twitch.Role) {
	url, err := s.oauth.AuthorizationURL(role)
	if err != nil {
		slog.Error("Could not build the authorization URL", "scope", "WEB", "role", role, "error", err)
		render(w, http.StatusInternalServerError, pageData{
			Title: "Something went wrong",
			Body:  "Could not start the authorization. Please try again.",
		})
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Twitch reports a declined consent screen here rather than as an error.
	if authErr := query.Get("error"); authErr != "" {
		render(w, http.StatusBadRequest, pageData{
			Title: "Authorization cancelled",
			Body:  "The authorization was not completed. Nothing has been saved.",
			Links: []link{{Href: "/", Label: "Start again"}},
		})
		return
	}

	code, state := query.Get("code"), query.Get("state")
	if code == "" || state == "" {
		render(w, http.StatusBadRequest, pageData{
			Title: "Invalid request",
			Body:  "This callback is missing its authorization code.",
		})
		return
	}

	user, err := s.oauth.Exchange(state, code)
	if err != nil {
		slog.Warn("Authorization exchange failed", "scope", "WEB", "error", err)
		render(w, http.StatusBadRequest, pageData{
			Title: "Authorization failed",
			Body:  "This authorization link is no longer valid. Please start again.",
			Links: []link{{Href: "/", Label: "Start again"}},
		})
		return
	}

	if err := s.register(r.Context(), user); err != nil {
		slog.Error("Could not save an authorization",
			"scope", "WEB", "role", user.Role, "login", user.Login, "error", err)
		render(w, http.StatusInternalServerError, pageData{
			Title: "Something went wrong",
			Body:  "Your authorization could not be saved. Please try again.",
		})
		return
	}

	slog.Info("Authorization complete",
		"scope", "WEB", "role", user.Role, "login", user.Login, "user_id", user.ID)

	body := fmt.Sprintf("MitoBoat is now set up for %s.", user.DisplayName)
	if user.Role == twitch.RoleBot {
		body = fmt.Sprintf("The bot will now post as %s. Restart the bot to pick up the new token.", user.DisplayName)
	}
	render(w, http.StatusOK, pageData{Title: "All set", Body: body})
}

func (s *Server) register(ctx context.Context, user *twitch.AuthorizedUser) error {
	if user.Role == twitch.RoleBot {
		return s.registrar.RegisterBot(ctx, user)
	}
	return s.registrar.RegisterStreamer(ctx, user)
}

// securityHeaders sets conservative defaults on every response. The pages are
// self contained, so nothing needs to be loaded from anywhere else.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
