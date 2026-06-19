package routing

import (
	"context"

	"github.com/xtls/xray-core/app/router/command"
	"google.golang.org/grpc"
)

// routingClient 是 Service 所需的 RoutingService 方法子集，command.RoutingServiceClient 满足之。
type routingClient interface {
	ListRule(ctx context.Context, in *command.ListRuleRequest, opts ...grpc.CallOption) (*command.ListRuleResponse, error)
}

// Service 在 RoutingService gRPC 客户端之上提供只读路由查询方法。
type Service struct {
	client routingClient
}

// NewService 用给定的 RoutingService 客户端构造 Service。
func NewService(client routingClient) *Service {
	return &Service{client: client}
}

// ListRules 返回当前路由规则列表（不订阅，返回快照）。
func (s *Service) ListRules(ctx context.Context) ([]RuleJSON, error) {
	resp, err := s.client.ListRule(ctx, &command.ListRuleRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]RuleJSON, 0, len(resp.Rules))
	for _, r := range resp.Rules {
		out = append(out, RuleJSON{Tag: r.Tag, RuleTag: r.RuleTag})
	}
	return out, nil
}
