package routes

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	authcontroller2 "sso/app/http/controllers/api/authcontroller"
	"sso/app/http/controllers/api/permissioncontroller"
	"sso/app/http/controllers/api/rolecontroller"
	"sso/app/http/controllers/api/usercontroller"
	"sso/app/http/controllers/authcontroller"
	auth2 "sso/app/http/middlewares/auth"
	"sso/app/http/middlewares/i18n"
	"sso/config/env"
)

func Init(router *gin.Engine, env *env.Env) {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AddAllowHeaders("X-Request-Token")
	router.Use(cors.New(config))
	router.Use(sessions.Sessions("sso", env.SessionStore()), i18n.I18nMiddleware(env))

	router.Static("/assets", "resources/css")
	router.Static("/images", "resources/images")
	router.LoadHTMLGlob("resources/views/*")
	// for debug
	//router.LoadHTMLGlob("/Users/congcong/uco/sso/resources/views/*")

	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"code": 404, "message": "Page not found"})
	})

	auth := authcontroller.New(env)

	guest := router.Group("/", auth2.GuestMiddleware(env))
	{
		guest.GET("/login", auth.LoginForm)
		guest.POST("/login", auth.Login)
	}

	authRouter := router.Group("/auth", auth2.SessionMiddleware(env))
	{
		authRouter.GET("/select_system", auth.SelectSystem)
		authRouter.GET("/logout", auth.Logout)
	}

	router.POST("/access_token", auth.AccessToken)

	apiGroup := router.Group("/api")
	{
		apiAuth := authcontroller2.New(env)
		apiGroup.POST("/login", apiAuth.Login)

		api := apiGroup.Group("/", auth2.ApiMiddleware(env))
		api.POST("/user/info", apiAuth.Info)
		api.POST("/logout", apiAuth.Logout)

		role := rolecontroller.NewRoleController(env)
		api.GET("/all_roles", role.All)
		api.GET("/roles", role.Index)
		api.POST("/roles", role.Store)
		api.GET("/roles/:role", role.Show)
		api.PUT("/roles/:role", role.Update)
		api.DELETE("/roles/:role", role.Destroy)

		permissions := permissioncontroller.NewPermissionController(env)
		api.GET("/permissions_by_group", permissions.GetByGroups)
		api.GET("/get_permission_projects", permissions.GetPermissionProjects)
		api.GET("/permissions", permissions.Index)
		api.POST("/permissions", permissions.Store)
		api.GET("/permissions/:permission", permissions.Show)
		api.PUT("/permissions/:permission", permissions.Update)
		api.DELETE("/permissions/:permission", permissions.Destroy)

		user := usercontroller.NewUserController(env)
		api.POST("/users/:user/force_logout", user.ForceLogout)
		api.POST("/users/:user/sync_roles", user.SyncRoles)
		api.GET("/users", user.Index)
		api.POST("/users", user.Store)
		api.GET("/users/:user", user.Show)
		api.PUT("/users/:user", user.Update)
		api.DELETE("/users/:user", user.Destroy)
	}
}
