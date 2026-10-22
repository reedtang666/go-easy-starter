package user

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/reedtang666/go-easy-starter/internal/dao"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
	"golang.org/x/crypto/bcrypt"
	"regexp"
	"strings"
	"time"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_\-]{3,32}$`)

type sUser struct{ store *dao.Store }

func New(s *dao.Store) *sUser { return &sUser{store: s} }
func NormalizeUsername(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !usernamePattern.MatchString(name) {
		return "", response.Fail(400, "USERNAME_INVALID", "用户名需为3至32位字母、数字、下划线或连字符")
	}
	return name, nil
}
func ValidatePassword(p string) error {
	if len(p) < 8 || len(p) > 72 {
		return response.Fail(400, "PASSWORD_INVALID", "密码需为8至72字节")
	}
	return nil
}
func (s *sUser) Register(ctx context.Context, username, password string) (*view.User, error) {
	name, e := NormalizeUsername(username)
	if e != nil {
		return nil, e
	}
	if e = ValidatePassword(password); e != nil {
		return nil, e
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return nil, e
	}
	id, e := s.store.InsertID(ctx, dao.Users.Table(), g.Map{"username": name, "password_hash": string(hash)})
	if e != nil {
		return nil, dao.Error(e)
	}
	return s.Get(ctx, id)
}
func (s *sUser) Get(ctx context.Context, id int64) (*view.User, error) {
	var u *view.User
	e := s.store.Users(ctx).Where("id", id).Scan(&u)
	if e != nil {
		return nil, e
	}
	if u == nil {
		return nil, response.Fail(404, "USER_NOT_FOUND", "用户不存在")
	}
	return u, nil
}
func (s *sUser) Authenticate(ctx context.Context, username, password string) (*view.User, error) {
	name := strings.ToLower(strings.TrimSpace(username))
	var u *view.User
	e := s.store.Users(ctx).Where("username", name).Scan(&u)
	if e != nil {
		return nil, e
	}
	if u == nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, response.Fail(401, "CREDENTIALS_INVALID", "用户名或密码错误")
	}
	if u.Status != 1 {
		return nil, response.Fail(403, "USER_DISABLED", "账号已停用")
	}
	return u, nil
}
func (s *sUser) Update(ctx context.Context, id int64, nickname string, avatarID *int64) (*view.User, error) {
	if len([]rune(nickname)) > 80 {
		return nil, response.Fail(400, "NICKNAME_INVALID", "昵称过长")
	}
	e := s.store.DB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		data := g.Map{"nickname": nickname, "updated_at": time.Now()}
		if avatarID != nil {
			if *avatarID > 0 {
				var f *view.Attachment
				if e := tx.Model("attachments").Ctx(ctx).Where("id", *avatarID).Where("owner_id", id).LockUpdate().Scan(&f); e != nil {
					return e
				}
				if f == nil {
					return response.Fail(404, "FILE_NOT_FOUND", "附件不存在")
				}
				data["avatar_id"] = *avatarID
			} else {
				data["avatar_id"] = gdb.Raw("NULL")
			}
		}
		_, e := tx.Model("users").Ctx(ctx).Where("id", id).Data(data).Update()
		return dao.Error(e)
	})
	if e != nil {
		return nil, e
	}
	return s.Get(ctx, id)
}
func (s *sUser) ChangePassword(ctx context.Context, id int64, old, new string) error {
	if e := ValidatePassword(new); e != nil {
		return e
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(new), bcrypt.DefaultCost)
	if e != nil {
		return e
	}
	return s.store.DB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var u *view.User
		if e := tx.Model("users").Ctx(ctx).Where("id", id).LockUpdate().Scan(&u); e != nil {
			return e
		}
		if u == nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(old)) != nil {
			return response.Fail(401, "CREDENTIALS_INVALID", "原密码错误")
		}
		_, e := tx.Model("users").Ctx(ctx).Where("id", id).Data(g.Map{"password_hash": string(hash), "credential_version": gdb.Raw("credential_version + 1"), "updated_at": time.Now()}).Update()
		return e
	})
}
