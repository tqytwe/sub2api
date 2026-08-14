package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func adminActorID(c *gin.Context) int64 {
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	return subject.UserID
}
