package wxoslogin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
	"yingce/backend/internal/auth"
	"yingce/backend/internal/kernel"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

type testHost struct{}

func (testHost) RequireAdmin(user *model.User) error {
	if user == nil || user.Role != model.UserRoleAdmin {
		return kernel.Forbidden("管理员权限不足")
	}
	return nil
}
func (testHost) EncryptSecret(value string) (string, error) { return "encrypted:" + value, nil }
func (testHost) DecryptSecret(value string) (string, error) {
	return strings.TrimPrefix(value, "encrypted:"), nil
}

func fixture(t *testing.T) (*Service, *auth.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+kernel.NewID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserIdentity{}, &model.CreditAccount{}, &model.AuthSession{}, &model.OAuthState{}, &model.LoginLog{}, &model.SystemSetting{}, &model.PluginPlatformState{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	repo := repository.New(db)
	login := auth.New(repo, nil, nil)
	plugin := New(repo, testHost{}, login)
	admin := &model.User{ID: "admin", Username: "admin", DisplayName: "同名用户", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.UpdateWxosSetting(admin, WxosSettingRequest{Issuer: "wxos-test", Secret: strings.Repeat("s", 43)}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.PluginPlatformState{PluginID: ID, Available: true}).Error; err != nil {
		t.Fatal(err)
	}
	return plugin, login, db
}

func request(subject string, mutate func(*wxosClaims)) WxosLoginRequest {
	verifier := strings.Repeat("v", 43)
	proof := sha256.Sum256([]byte(verifier))
	now := time.Now().Unix()
	nonce := sha256.Sum256([]byte(kernel.NewID()))
	claims := wxosClaims{Version: 1, Issuer: "wxos-test", Audience: "yingce:wxos", Subject: subject, DisplayName: "同名用户", Challenge: base64.RawURLEncoding.EncodeToString(proof[:]), Nonce: base64.RawURLEncoding.EncodeToString(nonce[:]), IssuedAt: now, ExpiresAt: now + 60}
	if mutate != nil {
		mutate(&claims)
	}
	payload, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(strings.Repeat("s", 43)))
	mac.Write([]byte("wxos-yingce-v1\n" + encoded))
	return WxosLoginRequest{Ticket: encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), Verifier: verifier}
}

func TestLoginCreatesUniqueIdentityWalletAndPreservesOtherSessions(t *testing.T) {
	plugin, login, db := fixture(t)
	first, err := plugin.LoginWxos(request("user-one", nil), auth.LoginContext{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := plugin.LoginWxos(request("user-one", nil), auth.LoginContext{})
	if err != nil {
		t.Fatal(err)
	}
	other, err := plugin.LoginWxos(request("user-two", nil), auth.LoginContext{})
	if err != nil {
		t.Fatal(err)
	}
	if first.User.ID == "admin" || first.User.ID != second.User.ID || first.User.ID == other.User.ID || first.User.Role != model.UserRoleUser {
		t.Fatal("incorrect account association")
	}
	var wallets int64
	db.Model(&model.CreditAccount{}).Count(&wallets)
	if wallets != 2 {
		t.Fatalf("wallet count = %d", wallets)
	}
	if err := login.Logout(first.Session); err != nil {
		t.Fatal(err)
	}
	if _, err := login.CurrentUser(first.Session); err == nil {
		t.Fatal("revoked embedded session remains valid")
	}
	if _, err := login.CurrentUser(second.Session); err != nil {
		t.Fatal("other session revoked:", err)
	}
	hash, _ := auth.HashPassword("standalone-password")
	if err := db.Model(&model.User{}).Where("id = ?", first.User.ID).Update("password_hash", hash).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := login.Login(auth.LoginRequest{Username: first.User.Username, Password: "standalone-password"}); err != nil {
		t.Fatal("native login failed:", err)
	}
}

func TestRejectReplayInvalidProofExpiredTicketAndWrongIssuer(t *testing.T) {
	plugin, _, db := fixture(t)
	valid := request("one", nil)
	if _, err := plugin.LoginWxos(valid, auth.LoginContext{}); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.LoginWxos(valid, auth.LoginContext{}); err == nil {
		t.Fatal("replay accepted")
	}
	cases := []WxosLoginRequest{
		request("two", func(c *wxosClaims) { c.Issuer = "other" }),
		request("two", func(c *wxosClaims) { c.ExpiresAt = time.Now().Unix() - 1 }),
		request("two", func(c *wxosClaims) { c.Audience = "other" }),
		request("two", func(c *wxosClaims) { c.IssuedAt += 120; c.ExpiresAt += 120 }),
		request("two", func(c *wxosClaims) { c.ExpiresAt += 100 }),
		request("two", func(c *wxosClaims) { c.Challenge = strings.Repeat("x", 43) }),
	}
	badSignature := request("two", nil)
	badSignature.Ticket += "A"
	cases = append(cases, badSignature)
	for _, req := range cases {
		if _, err := plugin.LoginWxos(req, auth.LoginContext{}); err == nil {
			t.Fatal("invalid ticket accepted")
		}
	}
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count != 2 {
		t.Fatal("invalid request created user")
	}
}

func TestDisabledPluginAndDisabledUserCannotLogin(t *testing.T) {
	plugin, _, db := fixture(t)
	if err := db.Model(&model.PluginPlatformState{}).Where("plugin_id = ?", ID).Update("available", false).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.LoginWxos(request("one", nil), auth.LoginContext{}); err == nil {
		t.Fatal("disabled plugin accepted login")
	}
	db.Model(&model.PluginPlatformState{}).Where("plugin_id = ?", ID).Update("available", true)
	first, err := plugin.LoginWxos(request("one", nil), auth.LoginContext{})
	if err != nil {
		t.Fatal(err)
	}
	db.Model(&model.User{}).Where("id = ?", first.User.ID).Update("status", model.UserStatusDisabled)
	if _, err := plugin.LoginWxos(request("one", nil), auth.LoginContext{}); err == nil {
		t.Fatal("disabled user received session")
	}
	if _, err := plugin.UpdateWxosSetting(&model.User{Role: model.UserRoleUser}, WxosSettingRequest{}); err == nil {
		t.Fatal("non-admin changed config")
	}
	if _, err := plugin.UpdateWxosSetting(&model.User{Role: model.UserRoleAdmin}, WxosSettingRequest{Issuer: "other"}); err == nil {
		t.Fatal("issuer changed")
	}
}
