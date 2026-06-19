// Package proxyman 在 Xray gRPC HandlerService 之上提供面向 HTTP 的 inbound 用户管理封装。
package proxyman

// UserJSON 表示某 inbound 下的一个用户（当前仅解析 VLESS 账户细节）。
type UserJSON struct {
	Email string `json:"email"`
	Level uint32 `json:"level"`
	// ID/Flow 仅当账户为 VLESS 且能成功解析时填充，否则为空。
	ID   string `json:"id,omitempty"`
	Flow string `json:"flow,omitempty"`
}

// AddUserRequest 是新增 VLESS 用户的入参（来自 HTTP JSON body）。
type AddUserRequest struct {
	Tag   string `json:"tag"`
	Email string `json:"email"`
	ID    string `json:"id"`
	Flow  string `json:"flow"`
	Level uint32 `json:"level"`
}

// CountJSON 表示某 inbound 下的用户数。
type CountJSON struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}
