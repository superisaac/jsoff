package jsoffnet

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"net/http"
	"strings"
	"time"
)

type AuthInfo struct {
	Username string
	Settings map[string]interface{}
}

type authInfoKeyType int

var authInfoKey authInfoKeyType

func AuthInfoFromContext(ctx context.Context) (*AuthInfo, bool) {
	if v := ctx.Value(authInfoKey); v != nil {
		authinfo, ok := v.(*AuthInfo)
		return authinfo, ok
	}
	return nil, false
}

type BasicAuthConfig struct {
	Username string                 `yaml:"username" json:"username"`
	Password string                 `yaml:"password" json:"password"`
	Settings map[string]interface{} `yaml:"settings,omitempty" json:"settings.omitempty"`
}

type BearerAuthConfig struct {
	Token string `yaml:"token" json:"token"`

	// username attached to request when token authorized
	Username string                 `yaml:"username,omitempty" json:"username,omitempty"`
	Settings map[string]interface{} `yaml:"settings,omitempty" json:"settings.omitempty"`
}

type JwtAuthConfig struct {
	Secret string `yaml:"secret" json:"secret"`
}

type AuthConfig struct {
	Basic  []BasicAuthConfig  `yaml:"basic,omitempty" json:"basic,omitempty"`
	Bearer []BearerAuthConfig `yaml:"bearer,omitempty" json:"bearer,omitempty"`
	Jwt    *JwtAuthConfig     `yaml:"jwt,omitempty" json:"jwt,omitempty"`
}

type jwtClaims struct {
	Username string                 `json:"username"`
	Settings map[string]interface{} `json:"settings,omitempty"`
	jwt.StandardClaims
}

// Auth handler
type AuthHandler struct {
	authConfig *AuthConfig
	next       http.Handler
	jwtCache   *lru.Cache
}

func NewAuthHandler(authConfig *AuthConfig, next http.Handler) *AuthHandler {
	cache, err := lru.New(100)
	if err != nil {
		panic(err)
	}
	return &AuthHandler{
		authConfig: authConfig,
		jwtCache:   cache,
		next:       next,
	}
}

func (handler AuthHandler) TryAuth(r *http.Request) (*AuthInfo, bool) {
	if handler.authConfig == nil {
		return nil, true
	}

	if handler.authConfig.Jwt != nil && handler.authConfig.Jwt.Secret != "" {
		if authInfo, ok := handler.jwtAuth(handler.authConfig.Jwt, r); ok {
			return authInfo, true
		}
	}

	if len(handler.authConfig.Basic) > 0 {
		if username, password, ok := r.BasicAuth(); ok {
			for _, basicCfg := range handler.authConfig.Basic {
				if basicCfg.Username == username && basicCfg.Password == password {
					return &AuthInfo{
						Username: username,
						Settings: basicCfg.Settings}, true
				}
			}
		}
	}

	if len(handler.authConfig.Bearer) > 0 {
		authHeader := r.Header.Get("Authorization")
		for _, bearerCfg := range handler.authConfig.Bearer {
			expect := fmt.Sprintf("Bearer %s", bearerCfg.Token)
			if authHeader == expect {
				username := bearerCfg.Username
				if username == "" {
					username = bearerCfg.Token
				}
				return &AuthInfo{
					Username: username,
					Settings: bearerCfg.Settings}, true
			}
		}
	}

	return nil, false
}

func (handler *AuthHandler) jwtAuth(jwtCfg *JwtAuthConfig, r *http.Request) (*AuthInfo, bool) {
	// refers to https://qvault.io/cryptography/jwts-in-golang/
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, false
	}
	if arr := strings.SplitN(authHeader, " ", 2); len(arr) <= 2 && arr[0] == "Bearer" {
		var claims *jwtClaims
		fromCache := false

		if cached, ok := handler.jwtCache.Get(authHeader); ok {
			claims, _ = cached.(*jwtClaims)
			fromCache = true
		} else {
			jwtFromHeader := arr[1]
			token, err := jwt.ParseWithClaims(
				jwtFromHeader,
				&jwtClaims{},
				func(token *jwt.Token) (interface{}, error) {
					return []byte(jwtCfg.Secret), nil
				},
			)

			if err != nil {
				Logger(r).Warnf("jwt auth error %s", err)
				return nil, false
			}
			claims, ok = token.Claims.(*jwtClaims)
			if !ok {
				return nil, false
			}
		}
		// check expiration
		if claims.ExpiresAt < time.Now().UTC().Unix() {
			Logger(r).Warnf("claims expired %s", authHeader)
			if fromCache {
				handler.jwtCache.Remove(authHeader)
			}
			return nil, false
		}
		if !fromCache {
			handler.jwtCache.Add(authHeader, claims)
		}
		return &AuthInfo{
			Username: claims.Username,
			Settings: claims.Settings}, true
	}
	return nil, false
}

func (handler *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if authInfo, ok := handler.TryAuth(r); ok {
		if authInfo != nil {
			ctx := context.WithValue(r.Context(), authInfoKey, authInfo)
			handler.next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			handler.next.ServeHTTP(w, r)
		}
	} else {
		w.WriteHeader(401)
		w.Write([]byte("auth failed!\n"))
	}
}

// Auth config
func (handler *AuthConfig) ValidateValues() error {
	if handler == nil {
		return nil
	}

	if len(handler.Bearer) > 0 {
		for _, bearerCfg := range handler.Bearer {
			if bearerCfg.Token == "" {
				return errors.New("bearer token is empty")
			}
		}
	}

	if len(handler.Basic) > 0 {
		for _, basicCfg := range handler.Basic {
			if basicCfg.Username == "" || basicCfg.Password == "" {
				return errors.New("basic username or password are empty")
			}
		}
	}

	if handler.Jwt != nil && handler.Jwt.Secret == "" {
		return errors.New("jwt has no secret")
	}
	return nil
}
