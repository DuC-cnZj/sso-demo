package user_repository

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"sso/app/models"
	"sso/config/env"
	"sso/utils/helper"
	"time"
)

var (
	ErrorTokenExpired = errors.New("token expired")
)

type UserRepositoryImp interface {
	FindByEmail(string, ...interface{}) (*models.User, error)
	GeneratePwd(string) (string, error)
	Create(*models.User) error
	FindById(uint) (*models.User, error)
	SyncRoles(*models.User, []*models.Role) error
	ForceLogout(*models.User)
	GenerateApiToken(*models.User) string
	GenerateLogoutToken(*models.User)
	GenerateAccessToken(*models.User) string
	FindByToken(string, bool) (*models.User, error)
	SyncPermissions(*models.User, []interface{}) error
	UpdateLastLoginAt(*models.User)
	FindWithRoles(int) (*models.User, error)
}

type UserRepository struct {
	env *env.Env
}

func NewUserRepository(env *env.Env) *UserRepository {
	return &UserRepository{
		env: env,
	}
}

func (repo *UserRepository) FindByEmail(email string, wheres ...interface{}) (*models.User, error) {
	user := &models.User{}

	log.Debug().Interface("email", wheres).Interface("e", email).Msg("da")
	if err := repo.env.GetDB().Where("email = ?", email).First(user, wheres...).Error; err != nil {
		log.Debug().Err(err).Msg("findByEmail")
		return nil, err
	}

	return user, nil
}

func (repo *UserRepository) GeneratePwd(pwd string) (string, error) {
	password, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(password), nil
}

func (repo *UserRepository) Create(user *models.User) error {
	if err := repo.env.GetDB().Create(user).Error; err != nil {
		return err
	}

	return nil
}

func (repo *UserRepository) FindById(id uint) (*models.User, error) {
	user := &models.User{}

	if err := repo.env.GetDB().Where("id = ?", id).First(user).Error; err != nil {
		log.Debug().Err(err).Msg("FindById")

		return nil, err
	}

	return user, nil
}

func (repo *UserRepository) SyncRoles(user *models.User, roles []*models.Role) error {
	return repo.env.DBTransaction(func(tx *gorm.DB) error {
		if tx.Model(user).Association("Roles").Clear().Error != nil {
			return tx.Model(user).Association("Roles").Clear().Error
		}

		tx.Model(user).Association("Roles").Append(toRoleInterfaceSlice(roles)...)

		return nil
	})
}

func (repo *UserRepository) ForceLogout(user *models.User) {
	repo.GenerateLogoutToken(user)
	repo.env.GetDB().Where("user_id = ?", user.ID).Delete(&models.ApiToken{})
}

func (repo *UserRepository) GenerateApiToken(user *models.User) string {
	var (
		try      int
		apiToken = &models.ApiToken{}
	)

	log.Debug().Interface("user", user).Msg("GenerateApiToken")

	data := map[string]interface{}{
		"user_id": user.ID,
	}

	for {
		if try > 10 {
			panic("error GenerateAccessToken try > 10")
		}

		token := helper.RandomString(64)

		data["api_token"] = token

		exists := repo.env.GetDB().First(apiToken, data)
		if exists.Error != nil && errors.Is(gorm.ErrRecordNotFound, exists.Error) {
			apiToken.ApiToken = token
			apiToken.UserID = user.ID
			repo.env.GetDB().Create(apiToken)
			return token
		}

		try++
	}
}

func (repo *UserRepository) GenerateLogoutToken(user *models.User) {
	str := helper.RandomString(64)
	repo.env.GetDB().Model(user).Updates(map[string]interface{}{"logout_token": str})
}

func (repo *UserRepository) GenerateAccessToken(user *models.User) string {
	var (
		try   int
		str   string
		err   error
		reply interface{}
	)

	conn := repo.env.RedisPool().Get()
	defer conn.Close()

	for {
		if try > 10 {
			panic("error GenerateAccessToken try > 10")
		}

		str = helper.RandomString(64)

		reply, err = conn.Do("GET", str)
		if err == nil && reply == nil {
			if repo.env.Config().AccessTokenLifetime > 0 {
				reply, err = conn.Do("SETEX", str, repo.env.Config().AccessTokenLifetime, user.ID)
			} else {
				reply, err = conn.Do("SET", str, user.ID)
			}
			log.Debug().Err(err).Interface("reply", reply).Msg("GenerateAccessToken")
			if err == nil {
				return str
			}
		}

		try++
	}
}

func (repo *UserRepository) FindByToken(token string, updateLastUseAt bool) (*models.User, error) {
	var (
		apiToken = &models.ApiToken{}
	)

	if err := repo.env.GetDB().Preload("User").First(apiToken, map[string]interface{}{"api_token": token}).Error; err != nil {
		return nil, err
	}

	seconds := time.Second * time.Duration(repo.env.Config().ApiTokenLifetime)
	sub := apiToken.CreatedAt.Add(seconds).Sub(time.Now())
	log.Debug().Interface("sub", sub).Interface("sec", seconds).Interface("ca", apiToken.CreatedAt).Interface("now", time.Now()).Msg("dad")
	if sub < 0 {
		log.Debug().Msg("token 过期")
		repo.env.GetDB().Delete(apiToken)
		return nil, fmt.Errorf("%w %d", ErrorTokenExpired, sub)
	}

	if updateLastUseAt {
		now := time.Now()
		repo.env.GetDB().Model(apiToken).Updates(map[string]interface{}{"LastUseAt": &now})
	}

	return &apiToken.User, nil
}

func (repo *UserRepository) SyncPermissions(user *models.User, permissions []interface{}) error {
	return repo.env.DBTransaction(func(tx *gorm.DB) error {
		if tx.Model(user).Association("Permissions").Clear().Error != nil {
			return tx.Model(user).Association("Permissions").Clear().Error
		}

		tx.Model(user).Association("Permissions").Append(permissions...)

		return nil
	})
}

func (repo *UserRepository) UpdateLastLoginAt(user *models.User) {
	repo.env.GetDB().Model(user).Updates(map[string]interface{}{"last_login_at": time.Now()})
}

func (repo *UserRepository) FindWithRoles(id int) (*models.User, error) {
	user := &models.User{}

	if err := repo.env.GetDB().Preload("Roles").Where("id = ?", id).First(user).Error; err != nil {
		log.Debug().Err(err).Msg("FindWithRoles")
		return nil, err
	}

	return user, nil
}

func toRoleInterfaceSlice(slice interface{}) []interface{} {
	roles := slice.([]*models.Role)
	newS := make([]interface{}, len(roles))
	for i, v := range roles {
		newS[i] = v
	}

	return newS
}
