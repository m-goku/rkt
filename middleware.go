package rkt

import (
	"net/http"
	"strconv"

	"github.com/justinas/nosurf"
)

func (r *RKT) SessionLoad(next http.Handler) http.Handler {
	return r.Session.LoadAndSave(next)
}

func (c *RKT) NoSurf(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	secure, _ := strconv.ParseBool(c.config.cookie.secure)

	csrfHandler.ExemptGlob("/api/*")

	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Domain:   c.config.cookie.domain,
	})

	return csrfHandler
}
