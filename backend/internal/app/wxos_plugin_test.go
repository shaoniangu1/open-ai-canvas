package app

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func TestWxosPluginCenterControlsLoginAvailability(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+newID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.PluginPlatformState{}, &model.UserPluginState{}, &model.AdminAuditEvent{}, &model.SystemSetting{}, &model.LoginLog{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir()}
	if err := os.WriteFile(filepath.Join(svc.dataDir, ".settings-key"), []byte(strings.Repeat("k", 32)), 0600); err != nil {
		t.Fatal(err)
	}
	admin := &model.User{ID: "admin", Role: model.UserRoleAdmin}
	user := &model.User{ID: "member", Role: model.UserRoleUser}
	var found bool
	for _, plugin := range svc.Plugins() {
		if plugin.Manifest.ID == PluginWxosLogin {
			found = plugin.Management.Kind == "authentication" && plugin.Management.ActivationScope == PluginScopeSystem
		}
	}
	if !found {
		t.Fatal("built-in login plugin missing from plugin center")
	}
	states, err := svc.AdminPluginStates(admin)
	if err != nil {
		t.Fatal(err)
	}
	if states[PluginWxosLogin].EffectiveEnabled {
		t.Fatal("plugin must start disabled")
	}
	if _, err := svc.SetPluginPlatformAvailability(admin, PluginWxosLogin, true); err == nil {
		t.Fatal("unconfigured plugin enabled")
	}
	if _, err := svc.UpdateWxosSetting(admin, WxosSettingRequest{Issuer: "wxos-test", Secret: strings.Repeat("s", 43)}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetPluginPlatformAvailability(user, PluginWxosLogin, true); err == nil {
		t.Fatal("ordinary user enabled login plugin")
	}
	state, err := svc.SetPluginPlatformAvailability(admin, PluginWxosLogin, true)
	if err != nil || !state.EffectiveEnabled {
		t.Fatalf("enable failed: %#v %v", state, err)
	}
	if _, err := svc.SetUserPluginEnabled(user, PluginWxosLogin, true); err == nil {
		t.Fatal("login plugin allowed user toggle")
	}
	state, err = svc.SetPluginPlatformAvailability(admin, PluginWxosLogin, false)
	if err != nil || state.EffectiveEnabled {
		t.Fatalf("disable failed: %#v %v", state, err)
	}
	_, err = svc.LoginWxos(WxosLoginRequest{}, LoginContext{})
	if err == nil || !strings.Contains(err.Error(), "未启用") {
		t.Fatalf("disabled plugin login = %v", err)
	}
}
