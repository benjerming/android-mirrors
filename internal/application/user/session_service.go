// Package user 实现登录会话相关的应用服务。
//
// 主要职责：
//   - 处理"用户名登录"请求，必要时自动创建用户记录。
//   - 用 HMAC-SHA256 签发和校验会话 token，避免裸文本编码可被伪造。
//   - 给 HTTP 鉴权中间件提供"根据 token 还原当前登录用户"的能力。
package user

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"assassin-android-controller/internal/domain/repository"
	domainuser "assassin-android-controller/internal/domain/user"
)

var (
	// ErrInvalidUsername 表示用户名为空或只有空白字符，不能创建有效会话。
	ErrInvalidUsername = errors.New("session: invalid username")
	// ErrUnauthorized 表示 token 解析失败、签名不对或找不到对应用户，当前请求不再可信。
	ErrUnauthorized = errors.New("session: unauthorized")
	// ErrEmptySecret 表示创建服务时没传签名密钥，属于编程错误，必须在依赖装配阶段就发现。
	ErrEmptySecret = errors.New("session: secret must not be empty")
)

// SessionService 表示会话应用服务，负责登录建用户、签发带签名的 token 以及解析当前登录人。
type SessionService struct {
	userRepo repository.UserRepository // userRepo 用来查找或创建用户，让会话逻辑不直接碰数据库细节。
	secret   []byte                    // secret 表示 HMAC 签名密钥，从配置注入，不要硬编码。
}

// NewSessionService 用来创建会话服务，通常在应用启动装配依赖时调用。
//
// secret 不能为空——空密钥意味着任何人都能伪造任何用户的 token，等同于关掉鉴权。
func NewSessionService(userRepo repository.UserRepository, secret string) (*SessionService, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrEmptySecret
	}
	return &SessionService{
		userRepo: userRepo,
		secret:   []byte(secret),
	}, nil
}

// Login 用来处理用户名登录；同名用户已存在时直接复用，不再重复创建。
func (s *SessionService) Login(ctx context.Context, username string) (string, error) {
	normalized := strings.TrimSpace(username)
	if normalized == "" {
		return "", ErrInvalidUsername
	}

	foundUser, err := s.userRepo.FindByUsername(ctx, normalized)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return "", err
		}

		// 第一次登录就顺手把用户建出来，避免再多一次"先注册再登录"的交互。
		foundUser = &domainuser.User{
			Username: normalized,
		}
		if err := s.userRepo.Create(ctx, foundUser); err != nil {
			return "", err
		}
	}

	return s.signToken(foundUser), nil
}

// GetProfile 用来根据前端带来的 token 还原当前登录用户，供鉴权中间件和 /session/me 复用。
//
// token 校验过程：
//  1. 拆出 payload 和签名两段。
//  2. 用密钥重新算一遍 HMAC，常量时间比较，防止时序攻击。
//  3. 从 payload 解出用户编号，再去仓储查实际记录。
func (s *SessionService) GetProfile(ctx context.Context, token string) (*domainuser.User, error) {
	userID, err := s.parseToken(token)
	if err != nil {
		return nil, ErrUnauthorized
	}

	foundUser, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	return foundUser, nil
}

// signToken 用来把用户编号编码成"payload.signature"格式的 token。
//
// payload 选择只放 ID 而不放用户名，是为了避免改名后旧 token 仍然指向同一个人却显示成旧名字；
// 真正的用户名仍然要去数据库查，这样保证返回结果永远是最新的。
func (s *SessionService) signToken(entity *domainuser.User) string {
	payload := strconv.FormatUint(uint64(entity.ID), 10)
	signature := s.sign([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(signature)
}

// parseToken 用来把 token 解回用户编号。
//
// 这里只做阶段 1 最小实现：HMAC-SHA256 签名 + 用户编号载荷，没有过期时间和会话表。
// 后续要加过期、踢人功能时，可以把 payload 升级成 JSON 并加上过期戳，签名流程保持不变。
func (s *SessionService) parseToken(token string) (uint, error) {
	if strings.TrimSpace(token) == "" {
		return 0, ErrUnauthorized
	}

	// token 必须是 "payload.signature" 这两段。
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return 0, ErrUnauthorized
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, ErrUnauthorized
	}
	gotSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, ErrUnauthorized
	}

	// 用常量时间比较防止时序攻击：长度不一致或内容不一致都会被 hmac.Equal 安全地拒绝。
	wantSignature := s.sign(payload)
	if !hmac.Equal(gotSignature, wantSignature) {
		return 0, ErrUnauthorized
	}

	parsedID, err := strconv.ParseUint(string(payload), 10, 64)
	if err != nil {
		return 0, ErrUnauthorized
	}

	return uint(parsedID), nil
}

// sign 用来对给定 payload 计算 HMAC-SHA256 签名，签名密钥来自配置。
func (s *SessionService) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}
