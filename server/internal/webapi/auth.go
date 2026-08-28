package webapi

import (
	"net/http"
	"strings"

	"github.com/go-fuego/fuego"
	"github.com/golang-jwt/jwt/v5"
)

// sessionCookie holds the bearer token so the browser attaches it on its own
// and no script ever touches it.
//
// SameSite=Strict is load-bearing. Sending the token in a header used to make
// CSRF impossible, because a cross-origin page cannot set one. A cookie the
// browser sends automatically gives that back, and Strict is what takes it
// away again: the browser will not attach this on any cross-site request.
//
// Secure is deliberately unset. The device serves plain HTTP over the LAN, and
// a Secure cookie would simply never be stored, so nobody could log in.
const sessionCookie = "mundus_session"

type AuthStatusDTO struct {
	SetupRequired bool `json:"setup_required"`
}

type CredentialsInput struct {
	Password string `json:"password"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type TokenDTO struct {
	Token string `json:"token"`
}

type SessionDTO struct {
	Authenticated bool `json:"authenticated"`
}

// publicRoutes are reachable without a token: the UI has to be able to discover
// whether the device still needs setup, and to obtain a token in the first place.
var publicRoutes = map[string]bool{
	"/api/auth/status": true,
	"/api/auth/setup":  true,
	"/api/auth/login":  true,
	// Clearing the cookie must work even once the token inside it has expired,
	// otherwise a stale session can never be shaken off.
	"/api/auth/logout": true,
}

// requireAuth rejects any /api request without a valid bearer token. Tokens are
// read from the Authorization header only -- never a cookie -- so a cross-origin
// page cannot make the browser attach one on the user's behalf.
func requireAuth(d Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if publicRoutes[strings.TrimSuffix(r.URL.Path, "/")] {
				next.ServeHTTP(w, r)
				return
			}
			token := fuego.TokenFromHeader(r)
			if token == "" {
				if c, err := r.Cookie(sessionCookie); err == nil {
					token = c.Value
				}
			}
			if token == "" {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}
			if _, err := d.Security.ValidateToken(token); err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// issueToken mints a token, stores it as the session cookie, and also returns
// it so a script can use the Authorization header instead.
func issueToken(d Deps, w http.ResponseWriter) (TokenDTO, error) {
	token, err := d.Security.GenerateToken(jwt.MapClaims{"sub": "admin"})
	if err != nil {
		return TokenDTO{}, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(d.Security.ExpiresInterval.Seconds()),
	})
	return TokenDTO{Token: token}, nil
}

func registerAuth(api *fuego.Server, d Deps) {
	fuego.Get(api, "/auth/status", func(ctx fuego.ContextNoBody) (AuthStatusDTO, error) {
		return AuthStatusDTO{SetupRequired: !d.Auth.Configured()}, nil
	}, fuego.OptionOperationID("getAuthStatus"))

	fuego.Post(api, "/auth/setup", func(ctx fuego.ContextWithBody[CredentialsInput]) (TokenDTO, error) {
		body, err := ctx.Body()
		if err != nil {
			return TokenDTO{}, err
		}
		if err := d.Auth.Setup(body.Password); err != nil {
			return TokenDTO{}, fuego.BadRequestError{Title: err.Error()}
		}
		d.Log.Info("admin password configured")
		return issueToken(d, ctx.Response())
	}, fuego.OptionOperationID("setupAuth"))

	fuego.Post(api, "/auth/login", func(ctx fuego.ContextWithBody[CredentialsInput]) (TokenDTO, error) {
		body, err := ctx.Body()
		if err != nil {
			return TokenDTO{}, err
		}
		if !d.Auth.Verify(body.Password) {
			d.Log.Warn("failed login attempt")
			return TokenDTO{}, fuego.UnauthorizedError{Title: "incorrect password"}
		}
		return issueToken(d, ctx.Response())
	}, fuego.OptionOperationID("login"))

	fuego.Post(api, "/auth/logout", func(ctx fuego.ContextNoBody) (OK, error) {
		// The cookie is HttpOnly, so only the server can clear it.
		ctx.SetCookie(http.Cookie{
			Name: sessionCookie, Value: "", Path: "/",
			HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1,
		})
		return OK{OK: true}, nil
	}, fuego.OptionOperationID("logout"))

	// Deliberately trivial: the guard has already validated the token by the time
	// this runs, so reaching the handler at all is the answer. The web UI calls it
	// on load to find out whether a stored token still works.
	fuego.Get(api, "/auth/session", func(ctx fuego.ContextNoBody) (SessionDTO, error) {
		return SessionDTO{Authenticated: true}, nil
	}, fuego.OptionOperationID("getSession"))

	fuego.Post(api, "/auth/password", func(ctx fuego.ContextWithBody[ChangePasswordInput]) (TokenDTO, error) {
		body, err := ctx.Body()
		if err != nil {
			return TokenDTO{}, err
		}
		if err := d.Auth.ChangePassword(body.CurrentPassword, body.NewPassword); err != nil {
			return TokenDTO{}, fuego.BadRequestError{Title: err.Error()}
		}
		d.Log.Info("admin password changed")
		return issueToken(d, ctx.Response())
	}, fuego.OptionOperationID("changePassword"))
}
