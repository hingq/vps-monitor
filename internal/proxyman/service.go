package proxyman

import (
	"context"

	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/vless"
	"google.golang.org/grpc"
)

// handlerClient 是 Service 所需的 HandlerService 方法子集，command.HandlerServiceClient 满足之。
// 仅声明用到的方法，便于测试时构造轻量 fake。
type handlerClient interface {
	AlterInbound(ctx context.Context, in *command.AlterInboundRequest, opts ...grpc.CallOption) (*command.AlterInboundResponse, error)
	GetInboundUsers(ctx context.Context, in *command.GetInboundUserRequest, opts ...grpc.CallOption) (*command.GetInboundUserResponse, error)
	GetInboundUsersCount(ctx context.Context, in *command.GetInboundUserRequest, opts ...grpc.CallOption) (*command.GetInboundUsersCountResponse, error)
}

// Service 在 HandlerService gRPC 客户端之上提供 inbound 用户管理方法。
type Service struct {
	client handlerClient
}

// NewService 用给定的 HandlerService 客户端构造 Service。
func NewService(client handlerClient) *Service {
	return &Service{client: client}
}

// AddVLESSUser 向 tag 指定的 inbound 添加一个 VLESS 用户。
// VLESS 的 Encryption 固定为 "none"。
func (s *Service) AddVLESSUser(ctx context.Context, tag, email, id, flow string, level uint32) error {
	op := &command.AddUserOperation{User: &protocol.User{
		Level: level,
		Email: email,
		Account: serial.ToTypedMessage(&vless.Account{
			Id:         id,
			Flow:       flow,
			Encryption: "none",
		}),
	}}
	_, err := s.client.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag:       tag,
		Operation: serial.ToTypedMessage(op),
	})
	return err
}

// RemoveUser 从 tag 指定的 inbound 按 email 移除用户。
func (s *Service) RemoveUser(ctx context.Context, tag, email string) error {
	op := &command.RemoveUserOperation{Email: email}
	_, err := s.client.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag:       tag,
		Operation: serial.ToTypedMessage(op),
	})
	return err
}

// ListUsers 返回 tag 指定 inbound 下的用户列表。
func (s *Service) ListUsers(ctx context.Context, tag string) ([]UserJSON, error) {
	resp, err := s.client.GetInboundUsers(ctx, &command.GetInboundUserRequest{Tag: tag})
	if err != nil {
		return nil, err
	}
	out := make([]UserJSON, 0, len(resp.Users))
	for _, u := range resp.Users {
		out = append(out, userToJSON(u))
	}
	return out, nil
}

// CountUsers 返回 tag 指定 inbound 下的用户数。
func (s *Service) CountUsers(ctx context.Context, tag string) (*CountJSON, error) {
	resp, err := s.client.GetInboundUsersCount(ctx, &command.GetInboundUserRequest{Tag: tag})
	if err != nil {
		return nil, err
	}
	return &CountJSON{Tag: tag, Count: resp.Count}, nil
}

// userToJSON 将 protocol.User 转为 JSON 结构，尽量解析 VLESS 账户细节。
func userToJSON(u *protocol.User) UserJSON {
	j := UserJSON{Email: u.Email, Level: u.Level}
	if u.Account == nil {
		return j
	}
	inst, err := u.Account.GetInstance()
	if err != nil {
		return j
	}
	if acc, ok := inst.(*vless.Account); ok {
		j.ID = acc.Id
		j.Flow = acc.Flow
	}
	return j
}
