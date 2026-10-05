package httpapi

import (
	"errors"
	"net/http"

	"github.com/G6kco/CyberSpace/internal/auth"
	"github.com/gin-gonic/gin"
)

type authHandler struct {
	login       *auth.GoogleLogin
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
	c.Redirect(http.StatusAccepted, authURL)
}

func (h *authHandler) callBack(c *gin.Context) {
	loginCookie, sessionCookie := cookieNames(h.secure)
	browserCookie, err := c.Request.Cookie(loginCookie)
	if err != nil || c.Query("error") != "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "invlaid_login",
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
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "invalid_login",
				"message": "Login was not accepted",
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code" : "auth_unavailable",
			"message" : "Login is unavailable",
		})
	}
	
	http.SetCookie(c.Writer, &http.Cookie{
		Name: sessionCookie,
		Value: token,
		Path: "/",
		Secure: h.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusSeeOther, h.frontendURL)
}
