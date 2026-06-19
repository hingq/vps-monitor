package proxyman

import (
	"context"
	"errors"
	"testing"

	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/vless"
	"google.golang.org/grpc"
)

// fakeClient 是可配置的 handlerClient 测试替身。
type fakeClient struct {
	lastAlter *command.AlterInboundRequest
	users     *command.GetInboundUserResponse
	count     *command.GetInboundUsersCountResponse
	err       error // 非空时所有方法返回该错误
}

func (f *fakeClient) AlterInbound(_ context.Context, in *command.AlterInboundRequest, _ ...grpc.CallOption) (*command.AlterInboundResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.lastAlter = in
	return &command.AlterInboundResponse{}, nil
}

func (f *fakeClient) GetInboundUsers(_ context.Context, _ *command.GetInboundUserRequest, _ ...grpc.CallOption) (*command.GetInboundUserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.users, nil
}

func (f *fakeClient) GetInboundUsersCount(_ context.Context, _ *command.GetInboundUserRequest, _ ...grpc.CallOption) (*command.GetInboundUsersCountResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.count, nil
}

func TestAddVLESSUser(t *testing.T) {
	fake := &fakeClient{}
	svc := NewService(fake)

	err := svc.AddVLESSUser(context.Background(), "vless-in", "u1", "uuid-1", "xtls-rprx-vision", 0)
	if err != nil {
		t.Fatal(err)
	}
	if fake.lastAlter == nil || fake.lastAlter.Tag != "vless-in" {
		t.Fatalf("AlterInbound 未收到正确 tag: %+v", fake.lastAlter)
	}
	// 解析 operation，应为 AddUserOperation，且账户为 VLESS uuid-1。
	inst, err := fake.lastAlter.Operation.GetInstance()
	if err != nil {
		t.Fatal(err)
	}
	op, ok := inst.(*command.AddUserOperation)
	if !ok {
		t.Fatalf("operation 类型错误: %T", inst)
	}
	if op.User.Email != "u1" {
		t.Errorf("email 错误: %q", op.User.Email)
	}
	accInst, err := op.User.Account.GetInstance()
	if err != nil {
		t.Fatal(err)
	}
	acc := accInst.(*vless.Account)
	if acc.Id != "uuid-1" || acc.Flow != "xtls-rprx-vision" || acc.Encryption != "none" {
		t.Errorf("VLESS 账户字段错误: %+v", acc)
	}
}

func TestRemoveUser(t *testing.T) {
	fake := &fakeClient{}
	svc := NewService(fake)

	if err := svc.RemoveUser(context.Background(), "vless-in", "u1"); err != nil {
		t.Fatal(err)
	}
	inst, err := fake.lastAlter.Operation.GetInstance()
	if err != nil {
		t.Fatal(err)
	}
	op, ok := inst.(*command.RemoveUserOperation)
	if !ok {
		t.Fatalf("operation 类型错误: %T", inst)
	}
	if op.Email != "u1" {
		t.Errorf("email 错误: %q", op.Email)
	}
}

func TestListUsersParsesVLESS(t *testing.T) {
	fake := &fakeClient{users: &command.GetInboundUserResponse{Users: []*protocol.User{
		{Email: "u1", Level: 0, Account: serial.ToTypedMessage(&vless.Account{Id: "uuid-1", Flow: "xtls-rprx-vision"})},
		{Email: "u2"}, // 无账户，应只返回 email
	}}}
	svc := NewService(fake)

	users, err := svc.ListUsers(context.Background(), "vless-in")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("用户数错误: %d", len(users))
	}
	if users[0].ID != "uuid-1" || users[0].Flow != "xtls-rprx-vision" {
		t.Errorf("VLESS 解析错误: %+v", users[0])
	}
	if users[1].ID != "" {
		t.Errorf("无账户用户不应有 id: %+v", users[1])
	}
}

func TestCountUsers(t *testing.T) {
	fake := &fakeClient{count: &command.GetInboundUsersCountResponse{Count: 3}}
	svc := NewService(fake)

	got, err := svc.CountUsers(context.Background(), "vless-in")
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 3 || got.Tag != "vless-in" {
		t.Errorf("计数错误: %+v", got)
	}
}

func TestServiceError(t *testing.T) {
	svc := NewService(&fakeClient{err: errors.New("boom")})
	if err := svc.AddVLESSUser(context.Background(), "t", "e", "i", "", 0); err == nil {
		t.Error("应返回错误")
	}
	if _, err := svc.ListUsers(context.Background(), "t"); err == nil {
		t.Error("应返回错误")
	}
}
