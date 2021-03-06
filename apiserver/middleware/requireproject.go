package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nitrous-io/rise-server/apiserver/controllers"
	"github.com/nitrous-io/rise-server/apiserver/dbconn"
	"github.com/nitrous-io/rise-server/apiserver/models/project"
)

// RequireProject is a Gin middleware that:
// 1. checks that the "project_name" parameter in the path is the name of a
//    valid project, and
// 2. ensures that the project is owned by the current user.
func RequireProject(c *gin.Context) {
	u := controllers.CurrentUser(c)
	if u == nil {
		controllers.InternalServerError(c, nil)
		c.Abort()
		return
	}

	db, err := dbconn.DB()
	if err != nil {
		controllers.InternalServerError(c, err)
		c.Abort()
		return
	}

	name := c.Param("project_name")
	proj, err := project.FindByName(db, name)
	if err != nil {
		controllers.InternalServerError(c, err)
		c.Abort()
		return
	}

	if proj == nil || proj.UserID != u.ID {
		c.JSON(http.StatusNotFound, gin.H{
			"error":             "not_found",
			"error_description": "project could not be found",
		})
		c.Abort()
		return
	}

	c.Set(controllers.CurrentProjectKey, proj)

	c.Next()
}
