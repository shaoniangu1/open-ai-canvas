package wxoslogin

import "yingce/backend/internal/protocol"

func Manifest() protocol.Manifest {
	return protocol.Manifest{
		APIVersion: "yingce.plugin/v1",
		Metadata: protocol.Metadata{ID: ID, Name: "wxOS 登录", Version: "1.0.0", Vendor: "wxOS", Enabled: false, Installable: false,
			Description: "源码内置的 wxOS 无感登录插件；首次登录创建独立账号及空钱包，独立网页登录不受影响。"},
		Surfaces: []string{"settings"}, Permissions: []string{},
		Runtime: protocol.ManifestRuntime{Backend: "host:wxos-login", Web: "host"},
	}
}
