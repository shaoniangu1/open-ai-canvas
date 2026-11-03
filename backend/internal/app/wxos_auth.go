package app

import (
	"encoding/json"
	"yingce/backend/internal/model"
	"yingce/backend/internal/plugins/wxoslogin"
)

const PluginWxosLogin = wxoslogin.ID

func wxosLoginPluginView() PluginView {
	manifest := wxoslogin.Manifest()
	raw, _ := json.Marshal(manifest)
	return PluginView{Manifest: pluginManifestView(raw, manifest.Metadata, "bundled"), Source: "bundled", Status: "disabled"}
}

type WxosSettingRequest = wxoslogin.WxosSettingRequest
type PublicWxosSetting = wxoslogin.PublicWxosSetting
type WxosLoginRequest = wxoslogin.WxosLoginRequest

func (s *Service) wxosLoginPlugin() *wxoslogin.Service {
	return wxoslogin.New(s.repo, authHost{svc: s}, s.authDomain())
}

func (s *Service) AdminWxosSetting(actor *model.User) (*PublicWxosSetting, error) {
	return s.wxosLoginPlugin().AdminWxosSetting(actor)
}

func (s *Service) UpdateWxosSetting(actor *model.User, req WxosSettingRequest) (*PublicWxosSetting, error) {
	return s.wxosLoginPlugin().UpdateWxosSetting(actor, req)
}

func (s *Service) LoginWxos(req WxosLoginRequest, context LoginContext) (*AuthSessionResult, error) {
	return s.wxosLoginPlugin().LoginWxos(req, context)
}
