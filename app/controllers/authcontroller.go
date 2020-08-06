package authcontroller

import (
	"encoding/json"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/gomodule/redigo/redis"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"sso/app/http/middlewares/i18n"
	"sso/app/models"
	"sso/config/env"
	"sso/utils/form"
)

type LoginForm struct {
	UserName    string `form:"email" binding:"required"`
	Password    string `form:"password" binding:"required"`
	RedirectUrl string `form:"redirect_url"`
}

func New(env *env.Env) *authController {
	return &authController{env: env}
}

type authController struct {
	env *env.Env
}

func (*authController) LoginForm(ctx *gin.Context) {
	redirectUrl := ctx.Query("redirect_url")
	ctx.HTML(http.StatusOK, "login.tmpl", struct {
		RedirectUrl string
	}{
		RedirectUrl: redirectUrl,
	})
}

func (auth *authController) Login(ctx *gin.Context) {
	var loginForm LoginForm

	if err := ctx.ShouldBind(&loginForm); err != nil {
		errors := err.(validator.ValidationErrors)
		value, _ := ctx.Get(i18n.UserPreferLangKey)
		trans, _ := auth.env.GetUniversalTranslator().GetTranslator(value.(string))
		ctx.AbortWithStatusJSON(422, gin.H{"code": 422, "error": form.ErrorsToMap(errors, trans)})

		return
	}

	user := models.User{}.FindByEmail(loginForm.UserName, auth.env)
	if user == nil {
		ctx.JSON(404, gin.H{"code": 404, "error": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginForm.Password)); err != nil {
		log.Println(err)
		ctx.JSON(401, gin.H{"code": 401, "error": "email or password error!"})

		return
	}

	session := sessions.Default(ctx)
	bytes, _ := json.Marshal(user)
	session.Set("user", string(bytes))
	session.Save()

	if loginForm.RedirectUrl == "" {
		ctx.AbortWithStatusJSON(400, gin.H{})
		return
	}

	if loginForm.RedirectUrl == "" {
		ctx.Redirect(302, "/auth/select_system")
		return
	}

	token := user.GenerateAccessToken(auth.env)

	ctx.Redirect(302, loginForm.RedirectUrl+"?access_token="+token)
}

func (auth *authController) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	c.Redirect(302, "/login")
}

func (auth *authController) SelectSystem(c *gin.Context) {
	c.HTML(200, "select_system.tmpl", nil)
}

func (auth *authController) AccessToken(c *gin.Context) {
	var jsonData struct {
		AccessToken string `json:"access_token"`
	}
	err := c.BindJSON(&jsonData)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad request"})
		return
	}

	conn := auth.env.RedisPool().Get()

	defer conn.Close()

	id, err := redis.Int(conn.Do("GET", jsonData.AccessToken))
	log.Println(id, err)
	if err == nil {
		user := models.User{}.FindById(uint(id), auth.env)
		if user != nil {
			do, err := conn.Do("DEL", jsonData.AccessToken)
			log.Println(do, err)
			c.JSON(200, gin.H{"api_token": user.GenerateApiToken(auth.env, false)})
			return
		}
	}

	c.JSON(400, gin.H{"error": "bad request"})
}

func (auth *authController) Info(c *gin.Context) {
	token := c.Request.Header.Get("X-Request-Token")
	if token != "" {
		user := models.User{}.FindByToken(token, auth.env)
		if user != nil {
			c.JSON(200, gin.H{"data": user})
			return
		}
	}

	c.JSON(401, gin.H{"code": 401})

}
