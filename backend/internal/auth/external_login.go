package auth

import "yingce/backend/internal/model"

// 编译入宿主的受信登录插件完成身份验证后，仍通过统一会话入口检查账号状态并写登录记录。
func (s *Service) CreateExternalSession(user *model.User, event *model.LoginLog) (*AuthSessionResult, error) {
	return s.createAuthSession(user, event)
}

func (s *Service) RecordExternalLoginFailure(event *model.LoginLog, err error) error {
	return s.recordLoginFailure(event, err)
}
