// Package routing 在 Xray gRPC RoutingService 之上提供面向 HTTP 的只读封装。
package routing

// RuleJSON 表示一条路由规则的可读标识（ListRule 仅返回 tag 级信息）。
type RuleJSON struct {
	Tag     string `json:"tag"`
	RuleTag string `json:"rule_tag"`
}
