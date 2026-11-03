package wxoslogin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"yingce/backend/internal/auth"
	"yingce/backend/internal/kernel"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"

	"gorm.io/gorm"
)

const wxosSettingKey = "wxos_login"
const ID = "wxos-login"

type Host interface {
	RequireAdmin(*model.User) error
	EncryptSecret(string) (string, error)
	DecryptSecret(string) (string, error)
}

type Service struct {
	repo *repository.Repository
	host Host
	auth *auth.Service
}

func New(repo *repository.Repository, host Host, authentication *auth.Service) *Service {
	return &Service{repo: repo, host: host, auth: authentication}
}

func (s *Service) CheckConfigured() error {
	setting, err := s.readWxosSetting()
	if err != nil {
		return err
	}
	if setting.Issuer == "" || setting.Secret == "" {
		return kernel.BadAuthRequest("请先配置 wxOS 登录插件的身份来源编号和接入密钥")
	}
	return nil
}

var wxosIdentifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
var wxosProof = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var wxosSecret = regexp.MustCompile(`^[A-Za-z0-9_-]{43,128}$`)

type WxosSettingRequest struct {
	Issuer string `json:"issuer"`
	Secret string `json:"secret"`
}

type PublicWxosSetting struct {
	Issuer    string `json:"issuer"`
	HasSecret bool   `json:"hasSecret"`
}

type WxosLoginRequest struct {
	Ticket   string `json:"ticket"`
	Verifier string `json:"verifier"`
}

type wxosClaims struct {
	Version     int    `json:"version"`
	Issuer      string `json:"issuer"`
	Audience    string `json:"audience"`
	Subject     string `json:"subject"`
	DisplayName string `json:"displayName"`
	Challenge   string `json:"challenge"`
	Nonce       string `json:"nonce"`
	IssuedAt    int64  `json:"issuedAt"`
	ExpiresAt   int64  `json:"expiresAt"`
}

func (s *Service) readWxosSetting() (WxosSettingRequest, error) {
	var value WxosSettingRequest
	row, err := s.repo.SystemSetting(wxosSettingKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, nil
	}
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal([]byte(row.ValueJSON), &value); err != nil {
		return value, errors.New("wxOS 接入配置格式无效")
	}
	if value.Secret != "" {
		value.Secret, err = s.host.DecryptSecret(value.Secret)
	}
	return value, err
}

func (s *Service) AdminWxosSetting(actor *model.User) (*PublicWxosSetting, error) {
	if err := s.host.RequireAdmin(actor); err != nil {
		return nil, err
	}
	value, err := s.readWxosSetting()
	if err != nil {
		return nil, err
	}
	return &PublicWxosSetting{Issuer: value.Issuer, HasSecret: value.Secret != ""}, nil
}

func (s *Service) UpdateWxosSetting(actor *model.User, input WxosSettingRequest) (*PublicWxosSetting, error) {
	if err := s.host.RequireAdmin(actor); err != nil {
		return nil, err
	}
	current, err := s.readWxosSetting()
	if err != nil {
		return nil, err
	}
	input.Issuer, input.Secret = strings.TrimSpace(input.Issuer), strings.TrimSpace(input.Secret)
	if current.Issuer != "" && current.Issuer != input.Issuer {
		return nil, kernel.BadAuthRequest("已关联的 wxOS 身份来源不能修改")
	}
	if input.Secret == "" {
		input.Secret = current.Secret
	}
	if input.Issuer != "" && !wxosIdentifier.MatchString(input.Issuer) {
		return nil, kernel.BadAuthRequest("wxOS 身份来源编号无效")
	}
	if input.Secret != "" && !wxosSecret.MatchString(input.Secret) {
		return nil, kernel.BadAuthRequest("接入密钥须为 43–128 位字母、数字、下划线或短横线")
	}
	result := &PublicWxosSetting{Issuer: input.Issuer, HasSecret: input.Secret != ""}
	if input.Secret != "" {
		input.Secret, err = s.host.EncryptSecret(input.Secret)
		if err != nil {
			return nil, err
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveSystemSetting(&model.SystemSetting{Key: wxosSettingKey, ValueJSON: string(encoded), UpdatedBy: actor.ID}); err != nil {
		return nil, err
	}
	return result, nil
}

func verifyWxosTicket(req WxosLoginRequest, setting WxosSettingRequest, now time.Time) (*wxosClaims, error) {
	invalid := kernel.BadAuthRequest("wxOS 登录凭据无效或已过期，请重新进入工作流创作")
	if len(req.Ticket) > 4096 || !wxosProof.MatchString(req.Verifier) {
		return nil, invalid
	}
	parts := strings.Split(req.Ticket, ".")
	if len(parts) != 2 {
		return nil, invalid
	}
	mac := hmac.New(sha256.New, []byte(setting.Secret))
	mac.Write([]byte("wxos-yingce-v1\n" + parts[0]))
	signature, err := base64.RawURLEncoding.Strict().DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, invalid
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(parts[0])
	if err != nil {
		return nil, invalid
	}
	var claims wxosClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, invalid
	}
	proof := sha256.Sum256([]byte(req.Verifier))
	challenge := base64.RawURLEncoding.EncodeToString(proof[:])
	if claims.Version != 1 || claims.Issuer != setting.Issuer || claims.Audience != "yingce:wxos" ||
		!wxosIdentifier.MatchString(claims.Subject) || !wxosProof.MatchString(claims.Nonce) ||
		!hmac.Equal([]byte(claims.Challenge), []byte(challenge)) ||
		claims.IssuedAt > now.Unix()+5 || claims.ExpiresAt <= now.Unix() ||
		claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt-claims.IssuedAt > 60 || len([]rune(claims.DisplayName)) > 80 {
		return nil, invalid
	}
	return &claims, nil
}

func (s *Service) LoginWxos(req WxosLoginRequest, context auth.LoginContext) (result *auth.AuthSessionResult, err error) {
	event := auth.NewLoginLog("wxos", "login", context)
	defer func() {
		if err != nil {
			err = s.auth.RecordExternalLoginFailure(event, err)
		}
	}()
	setting, err := s.readWxosSetting()
	if err != nil {
		return nil, err
	}
	state, err := s.repo.PluginPlatformState(ID)
	if err != nil {
		return nil, err
	}
	if state == nil || !state.Available || setting.Secret == "" || setting.Issuer == "" {
		return nil, kernel.Forbidden("wxOS 登录未启用")
	}
	claims, err := verifyWxosTicket(req, setting, time.Now())
	if err != nil {
		return nil, err
	}
	count, err := s.repo.UserCount()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, kernel.Forbidden("请先创建影策管理员账号")
	}
	// 复用已有 OAuth 单次状态账本，不存原始票据和 wxOS 凭证。
	now := time.Now()
	used, err := s.repo.UseWxosLoginNonce(&model.OAuthState{
		ID: kernel.NewID(), Provider: "wxos", StateHash: auth.HashToken(claims.Issuer + ":" + claims.Nonce),
		ExpiresAt: time.Unix(claims.ExpiresAt, 0), UsedAt: &now, CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	if !used {
		return nil, kernel.BadAuthRequest("wxOS 登录凭据已使用，请重新进入工作流创作")
	}
	user, created, err := s.wxosUser(claims)
	if err != nil {
		return nil, err
	}
	if created {
		event.Source = "registration"
	}
	return s.auth.CreateExternalSession(user, event)
}

func (s *Service) wxosUser(claims *wxosClaims) (*model.User, bool, error) {
	subject := claims.Issuer + "/" + claims.Subject
	identity, err := s.repo.UserIdentity("wxos", subject)
	if err == nil {
		user, err := s.repo.User(identity.UserID)
		return user, false, err
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	user := &model.User{ID: kernel.NewID(), Username: "wxos_" + kernel.NewID(), DisplayName: auth.NormalizeDisplayName(claims.DisplayName, "wxOS 用户"), Role: model.UserRoleUser, Status: model.UserStatusActive}
	identity = &model.UserIdentity{ID: kernel.NewID(), UserID: user.ID, Provider: "wxos", Subject: subject}
	if err := s.repo.CreateOAuthUser(user, identity); err != nil {
		// 并发首次登录只允许唯一身份约束的胜出者创建账号和钱包。
		existing, lookupErr := s.repo.UserIdentity("wxos", subject)
		if lookupErr != nil {
			return nil, false, err
		}
		user, lookupErr = s.repo.User(existing.UserID)
		return user, false, lookupErr
	}
	return user, true, nil
}
