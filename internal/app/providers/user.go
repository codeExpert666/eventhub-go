package providers

import (
	"database/sql"

	userhandler "eventhub-go/internal/http/handler/user"
	"eventhub-go/internal/repository"
	repositorymysql "eventhub-go/internal/repository/mysql"
	usersvc "eventhub-go/internal/service/user"
)

// UserDeps 聚合 user 模块装配结果。
type UserDeps struct {
	Users   repository.UserRepository
	Roles   repository.RoleRepository
	Service *usersvc.Service
	Handler *userhandler.Handler
}

// ProviderUser 在数据库可用时创建 user 模块依赖。
func ProviderUser(database *sql.DB) UserDeps {
	if database == nil {
		return UserDeps{}
	}

	userRepo := repositorymysql.NewUserRepository(database)
	roleRepo := repositorymysql.NewRoleRepository(database)
	service := usersvc.NewService(userRepo, roleRepo)
	return UserDeps{
		Users:   userRepo,
		Roles:   roleRepo,
		Service: service,
		Handler: userhandler.NewHandler(service),
	}
}
