package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/G6kco/CyberSpace/internal/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type authHandler struct {
	login       *auth.GoogleLogin
	logger      *zap.Logger
	frontendURL string
	secure      bool
}

func cookieNames(secure bool) (login, session string) {
	if secure {
		return "__Host-cyberspace_login", "__Host-cyberspace_session"
	}
	return "cyberspace_login", "cyberspace_session"
}

func (h *authHandler) start(c *gin.Context) {
	authURL, browserSecret, err := h.login.Start(c.Request.Context())
	if err != nil {
		h.logger.Error("Cannot start login", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "auth_unavailable",
			"message": "Cannot start login",
		})
		return
	}

	loginCookie, _ := cookieNames(h.secure)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     loginCookie,
		Value:    browserSecret,
		Path:     "/",
		MaxAge:   300,
		Secure:   h.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, authURL)
}

func (h *authHandler) callback(c *gin.Context) {
	loginCookie, sessionCookie := cookieNames(h.secure)
	browserCookie, err := c.Request.Cookie(loginCookie)
	if err != nil || c.Query("error") != "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "invalid_login",
			"message": "Login failed or expired",
		})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     loginCookie,
		Path:     "/",
		MaxAge:   -1,
		Secure:   h.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	token, err := h.login.Complete(
		c.Request.Context(),
		c.Query("state"),
		browserCookie.Value,
		c.Query("code"),
	)

	if err != nil {
		if errors.Is(err, auth.ErrInvalidFlow) ||
			errors.Is(err, auth.ErrAccessDenied) {
			// The browser gets one generic message so that a failed login
			// reveals nothing about why. The reason goes to the log instead.
			h.logger.Warn("Login refused", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "invalid_login",
				"message": "Login was not accepted",
			})
			return
		}
		h.logger.Error("Login failed unexpectedly", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "auth_unavailable",
			"message": "Login is unavailable",
		})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.login.SessionTTL().Seconds()),
		Secure:   h.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusSeeOther, h.frontendURL)
}

func (h *authHandler) requireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, sessionCookie := cookieNames(h.secure)
		cookie, err := c.Request.Cookie(sessionCookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "unauthorized",
				"message": "Sign in required",
			})
			return
		}

		user, err := h.login.CurrentUser(
			c.Request.Context(),
			cookie.Value,
		)
		if errors.Is(err, auth.ErrUnauthenticated) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "unauthenticated",
				"message": "Sign in required",
			})
			return
		}
		if err != nil {
			h.logger.Error("Session lookup failed", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    "auth_unavailable",
				"message": "Authentication unavailable",
			})
			return
		}

		c.Set("currentUser", user)
		c.Set("sessionToken", cookie.Value)
		c.Next()
	}
}

func (h *authHandler) me(c *gin.Context) {
	user := c.MustGet("currentUser").(auth.User)
	c.JSON(http.StatusOK, user)
}

func (h *authHandler) logout(c *gin.Context) {
	// Logout changes server state through a cookie the browser attaches
	// automatically, so the origin is checked to reject cross-site callers.
	frontend, _ := url.Parse(h.frontendURL)
	allowedOrigin := frontend.Scheme + "://" + frontend.Host
	if !strings.EqualFold(c.GetHeader("Origin"), allowedOrigin) {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "bad_origin",
			"message": "Request origin not allowed",
		})
		return
	}

	token := c.MustGet("sessionToken").(string)
	if err := h.login.SignOut(c.Request.Context(), token); err != nil {
		h.logger.Error("Cannot sign out", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "auth_unavailable",
			"message": "Cannot sign out",
		})
		return
	}

	_, sessionCookie := cookieNames(h.secure)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Path:     "/",
		MaxAge:   -1,
		Secure:   h.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Status(http.StatusNoContent)
}
