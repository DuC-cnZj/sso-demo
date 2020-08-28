package usercontroller

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/rs/zerolog/log"
	"math"
	"sso/app/controllers/api"
	"sso/app/models"
	"sso/config/env"
	"sso/utils/exception"
	"sso/utils/form"
	"strconv"
	"strings"
)

type StoreInput struct {
	UserName string `form:"user_name"`
	Email    string `form:"email"`
	Password string `form:"password"`
}

type UpdateInput struct {
	UserName string `form:"user_name"`
	Email    string `form:"email"`
	//Password string `form:"password"`
}

type QueryInput struct {
	UserName string `form:"user_name"`
	Email    string `form:"email"`

	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Sort     string `form:"sort"`
}

type SyncInput struct {
	RoleIds []uint `form:"role_ids" json:"role_ids"`
}

type UserController struct {
	env *env.Env
	*api.AllRepo
}

func NewUserController(env *env.Env) *UserController {
	return &UserController{
		env:     env,
		AllRepo: api.NewAllRepo(env),
	}
}

func (user *UserController) Index(ctx *gin.Context) {
	var (
		query QueryInput
		count int
	)
	if err := ctx.ShouldBindQuery(&query); err != nil {
		exception.ValidateException(ctx, err, user.env)
		return
	}
	log.Debug().Interface("query", query).Msg("UserController.Index")
	var users []models.User
	if query.PageSize <= 0 {
		query.PageSize = 15
	}

	if query.Page <= 0 {
		query.Page = 1
	}

	switch strings.ToLower(query.Sort) {
	case "asc":
		query.Sort = "ASC"
	case "":
		fallthrough
	case "desc":
		fallthrough
	default:
		query.Sort = "DESC"
	}

	q := user.env.GetDB().Model(&users)
	if query.UserName != "" {
		q = q.Where("user_name like ?", "%"+query.UserName+"%")
	}
	if query.Email != "" {
		q = q.Where("email like ?", "%"+query.Email+"%")
	}
	offset := int(math.Max(float64((query.Page-1)*query.PageSize), 0))
	q.
		Preload("Roles.Permissions").
		Offset(offset).
		Limit(query.PageSize).
		Order("id " + query.Sort).
		Find(&users)
	if len(users) < query.PageSize {
		count = query.PageSize*(query.Page-1) + len(users)
	} else {
		countQuery := user.env.GetDB().Model(&models.Role{})
		if query.UserName != "" {
			countQuery = countQuery.Where("user_name like ?", "%"+query.UserName+"%")
		}
		if query.Email != "" {
			countQuery = countQuery.Where("email like ?", "%"+query.Email+"%")
		}
		countQuery.Count(&count)
	}
	ctx.JSON(200, gin.H{"code": 200, "data": users, "page": query.Page, "page_size": query.PageSize, "total": count})
}

func (user *UserController) Store(ctx *gin.Context) {
	var input StoreInput
	if err := ctx.ShouldBind(&input); err != nil {
		exception.ValidateException(ctx, err, user.env)
		log.Info().Err(err).Msg("UserController.Store")
		return
	}

	userByEmail := user.UserRepo.FindByEmail(input.Email)

	if userByEmail != nil {
		var errors = form.ValidateErrors{
			form.ValidateError{
				Field: "email",
				Msg:   "email exists",
			},
		}
		exception.ValidateException(ctx, errors, user.env)
		return
	}

	password, err := user.UserRepo.GeneratePwd(input.Password)

	if err != nil {
		log.Panic().Err(err).Msg("UserController.GenerateFromPassword")
		return
	}

	newUser := &models.User{
		UserName: input.Email,
		Email:    input.Email,
		Password: password,
	}

	if err := user.UserRepo.Create(newUser); err != nil {
		log.Panic().Err(err).Msg("UserController.GenerateFromPassword")
	}

	ctx.JSON(201, gin.H{"code": 201, "data": newUser})
}

func (user *UserController) Show(ctx *gin.Context) {
	var (
		u   *models.User
		err error
		id  int
	)

	if id, err = strconv.Atoi(ctx.Param("user")); err != nil {
		log.Panic().Err(err).Msg("UserController.Show")
		return
	}

	if u, _ = user.UserRepo.FindById(uint(id)); u == nil {
		exception.ModelNotFound(ctx, "user")
		return
	}

	ctx.JSON(200, gin.H{"data": u})
}

func (user *UserController) Update(ctx *gin.Context) {
	var input UpdateInput
	id, err := strconv.Atoi(ctx.Param("user"))
	if err != nil {
		log.Panic().Err(err).Msg("UserController.Update")
		return
	}
	if err := ctx.ShouldBind(&input); err != nil {
		exception.ValidateException(ctx, err, user.env)
		log.Panic().Err(err).Msg("UserController.Update")

		return
	}

	byId, _ := user.UserRepo.FindById(uint(id))
	if byId == nil {
		exception.ModelNotFound(ctx, "user")
		return
	}

	var updates = make([]interface{}, 0)
	if input.Email != "" {
		byEmail := user.UserRepo.FindByEmail(input.Email, user.env, "id <> ?", id)

		if byEmail != nil && byEmail.ID != uint(id) {
			var errors = form.ValidateErrors{
				form.ValidateError{
					Field: "email",
					Msg:   "email exists",
				},
			}
			exception.ValidateException(ctx, errors, user.env)
			return
		}

		updates = append(updates, "email", input.Email)
	}
	if input.UserName != "" {
		updates = append(updates, "user_name", input.UserName)
	}
	e := user.env.GetDB().Model(byId).Update(updates...)
	if e.Error != nil {
		log.Panic().Err(e.Error).Msg("UserController.Update")
	}

	ctx.JSON(200, gin.H{"code": 200, "data": byId})
}

func (user *UserController) Destroy(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("user"))
	if err != nil {
		log.Panic().Err(err).Msg("UserController.Destroy")
		return
	}

	byId, _ := user.UserRepo.FindById(uint(id))
	if byId == nil {
		exception.ModelNotFound(ctx, "user")
		return
	}
	if err := user.env.DBTransaction(func(tx *gorm.DB) error {
		tx.Model(byId).Association("Roles").Clear()
		tx.Model(byId).Association("Permissions").Clear()
		tx.Delete(byId)

		return nil
	}); err != nil {
		log.Panic().Err(err).Msg("UserController.Destroy")
	}

	ctx.JSON(204, nil)
}

func (user *UserController) SyncRoles(ctx *gin.Context) {
	var input SyncInput
	id, err := strconv.Atoi(ctx.Param("user"))
	if err != nil {
		log.Panic().Err(err).Msg("UserController.SyncRoles")

		return
	}
	log.Debug().Interface("SyncInput", input.RoleIds).Msg("UserController.SyncRoles")

	if err := ctx.ShouldBind(&input); err != nil {
		exception.ValidateException(ctx, err, user.env)
		log.Debug().Err(err).Msg("UserController.SyncRoles")
		return
	}
	byId, _ := user.UserRepo.FindById(uint(id))
	if byId == nil {
		exception.ModelNotFound(ctx, "user")
		return
	}

	roleByIds, err := user.RoleRepo.FindByIds(input.RoleIds)

	if err := user.UserRepo.SyncRoles(byId, roleByIds); err != nil {
		log.Panic().Err(err).Msg("UserController.SyncRoles")
	}

	roles, _ := user.UserRepo.FindWithRoles(id)

	ctx.JSON(200, gin.H{"data": roles})
}

func (user *UserController) ForceLogout(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("user"))
	if err != nil {
		log.Panic().Err(err).Msg("UserController.ForceLogout")
		return
	}

	byId, _ := user.UserRepo.FindById(uint(id))
	if byId == nil {
		exception.ModelNotFound(ctx, "user")
		return
	}

	user.UserRepo.ForceLogout(byId)
	ctx.JSON(200, gin.H{"data": true})
}