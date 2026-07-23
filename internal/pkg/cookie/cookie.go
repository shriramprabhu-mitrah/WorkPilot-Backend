package cookies

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetAccessToken(c *gin.Context, token string, maxAge int, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func SetRefreshToken(c *gin.Context, token string, maxAge int, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func Clear(c *gin.Context, secure bool) {
	names := []string{
		"access_token",
		"refresh_token",
	}

	for _, name := range names {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}
