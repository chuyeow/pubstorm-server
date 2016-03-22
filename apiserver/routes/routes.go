package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/nitrous-io/rise-server/apiserver/controllers/deployments"
	"github.com/nitrous-io/rise-server/apiserver/controllers/oauth"
	"github.com/nitrous-io/rise-server/apiserver/controllers/ping"
	"github.com/nitrous-io/rise-server/apiserver/controllers/projects"
	"github.com/nitrous-io/rise-server/apiserver/controllers/users"
	"github.com/nitrous-io/rise-server/apiserver/middleware"
)

func Draw(r *gin.Engine) {
	if gin.Mode() != gin.TestMode {
		r.Use(gin.Logger())
		r.Use(gin.Recovery())
	}

	r.GET("/ping", ping.Ping)
	r.POST("/users", users.Create)
	r.POST("/user/confirm", users.Confirm)
	r.POST("/user/confirm/resend", users.ResendConfirmationCode)
	r.POST("/oauth/token", oauth.CreateToken)

	{
		r2 := r.Group("/", middleware.RequireToken)
		r2.DELETE("/oauth/token", oauth.DestroyToken)
		r2.POST("/projects", projects.Create)

		{
			r3 := r2.Group("/projects/:project_name", middleware.RequireProject)
			r3.POST("/deployments", deployments.Create)
		}
	}
}