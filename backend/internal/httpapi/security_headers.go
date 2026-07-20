package httpapi

import "net/http"

const (
	defaultContentSecurityPolicy = "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; object-src 'none'; form-action 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' https: wss: ws:"
	defaultHSTSHeaderValue       = "max-age=31536000; includeSubDomains"
)

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()
		headers.Set("Content-Security-Policy", defaultContentSecurityPolicy)
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if requestIsSecure(r) {
			headers.Set("Strict-Transport-Security", defaultHSTSHeaderValue)
		}

		next.ServeHTTP(w, r)
	})
}
