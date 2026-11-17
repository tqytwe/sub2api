package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Preserve Gin header bookkeeping while exposing transport flush failures.
func FlushGatewayResponse(w gin.ResponseWriter) error {
	w.WriteHeaderNow()
	var target http.ResponseWriter = w
	for {
		if f, ok := target.(interface{ FlushError() error }); ok {
			return f.FlushError()
		}
		if u, ok := target.(interface{ Unwrap() http.ResponseWriter }); ok {
			target = u.Unwrap()
			continue
		}
		return http.NewResponseController(target).Flush()
	}
}
