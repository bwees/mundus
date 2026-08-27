package webapi

import (
	"net/http"
	"strings"

	"github.com/go-fuego/fuego"
	"github.com/golang-jwt/jwt/v5"
)

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

// publicRoutes are reachable without a token: the UI has to be able to discover
// whether the device still needs setup, and to obtain a token in the first place.
var publicRoutes = map[string]bool{
	"/api/auth/status": true,
	"/api/auth/setup":  true,
	"/api/auth/login":  true,
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

func (d Deps) issueToken() (TokenDTO, error) {
	token, err := d.Security.GenerateToken(jwt.MapClaims{"sub": "admin"})
	if err != nil {
		return TokenDTO{}, err
	}
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
		return d.issueToken()
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
		return d.issueToken()
	}, fuego.OptionOperationID("login"))

	fuego.Post(api, "/auth/password", func(ctx fuego.ContextWithBody[ChangePasswordInput]) (TokenDTO, error) {
		body, err := ctx.Body()
		if err != nil {
			return TokenDTO{}, err
		}
		if err := d.Auth.ChangePassword(body.CurrentPassword, body.NewPassword); err != nil {
			return TokenDTO{}, fuego.BadRequestError{Title: err.Error()}
		}
		d.Log.Info("admin password changed")
		return d.issueToken()
	}, fuego.OptionOperationID("changePassword"))
}
