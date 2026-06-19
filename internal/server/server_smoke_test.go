package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	proxymancmd "github.com/xtls/xray-core/app/proxyman/command"
	routercmd "github.com/xtls/xray-core/app/router/command"
	command "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"

	"vps/internal/hourly"
	"vps/internal/proxyman"
	"vps/internal/routing"
	"vps/internal/stats"
)

// fakeClient 实现 command.StatsServiceClient，返回固定数据用于路由冒烟测试。
type fakeClient struct{}

func (fakeClient) GetStats(_ context.Context, in *command.GetStatsRequest, _ ...grpc.CallOption) (*command.GetStatsResponse, error) {
	return &command.GetStatsResponse{Stat: &command.Stat{Name: in.Name, Value: 1024}}, nil
}
func (fakeClient) GetStatsOnline(_ context.Context, in *command.GetStatsRequest, _ ...grpc.CallOption) (*command.GetStatsResponse, error) {
	return &command.GetStatsResponse{Stat: &command.Stat{Name: in.Name, Value: 3}}, nil
}
func (fakeClient) QueryStats(_ context.Context, _ *command.QueryStatsRequest, _ ...grpc.CallOption) (*command.QueryStatsResponse, error) {
	return &command.QueryStatsResponse{Stat: []*command.Stat{
		{Name: "user>>>foo>>>traffic>>>uplink", Value: 1024},
		{Name: "user>>>foo>>>traffic>>>downlink", Value: 2048},
	}}, nil
}
func (fakeClient) GetSysStats(_ context.Context, _ *command.SysStatsRequest, _ ...grpc.CallOption) (*command.SysStatsResponse, error) {
	return &command.SysStatsResponse{Uptime: 42, Alloc: 1024}, nil
}
func (fakeClient) GetStatsOnlineIpList(_ context.Context, _ *command.GetStatsRequest, _ ...grpc.CallOption) (*command.GetStatsOnlineIpListResponse, error) {
	return &command.GetStatsOnlineIpListResponse{Ips: map[string]int64{"1.2.3.4": 1718700000}}, nil
}
func (fakeClient) GetAllOnlineUsers(_ context.Context, _ *command.GetAllOnlineUsersRequest, _ ...grpc.CallOption) (*command.GetAllOnlineUsersResponse, error) {
	return &command.GetAllOnlineUsersResponse{Users: []string{"foo"}}, nil
}

// fakeHandler 实现 proxyman 所需的 HandlerService 方法子集。
type fakeHandler struct{}

func (fakeHandler) AlterInbound(_ context.Context, _ *proxymancmd.AlterInboundRequest, _ ...grpc.CallOption) (*proxymancmd.AlterInboundResponse, error) {
	return &proxymancmd.AlterInboundResponse{}, nil
}
func (fakeHandler) GetInboundUsers(_ context.Context, _ *proxymancmd.GetInboundUserRequest, _ ...grpc.CallOption) (*proxymancmd.GetInboundUserResponse, error) {
	return &proxymancmd.GetInboundUserResponse{}, nil
}
func (fakeHandler) GetInboundUsersCount(_ context.Context, _ *proxymancmd.GetInboundUserRequest, _ ...grpc.CallOption) (*proxymancmd.GetInboundUsersCountResponse, error) {
	return &proxymancmd.GetInboundUsersCountResponse{Count: 2}, nil
}

// fakeRouting 实现 routing 所需的 RoutingService 方法子集。
type fakeRouting struct{}

func (fakeRouting) ListRule(_ context.Context, _ *routercmd.ListRuleRequest, _ ...grpc.CallOption) (*routercmd.ListRuleResponse, error) {
	return &routercmd.ListRuleResponse{Rules: []*routercmd.ListRuleItem{{Tag: "direct", RuleTag: "r1"}}}, nil
}

const testToken = "secret"

func TestRoutesSmoke(t *testing.T) {
	srv := New(
		stats.NewService(fakeClient{}),
		proxyman.NewService(fakeHandler{}),
		routing.NewService(fakeRouting{}),
		hourly.NewStore(filepath.Join(t.TempDir(), "hourly.json"), 30*24*time.Hour),
		testToken,
	)
	mux := srv.Routes()

	cases := []struct {
		method string
		path   string
		body   string
		token  string
		want   int
	}{
		{"GET", "/api/sysstats", "", "", 200},
		{"GET", "/api/stats?name=user>>>foo>>>traffic>>>uplink", "", "", 200},
		{"GET", "/api/stats", "", "", 400}, // 缺 name
		{"GET", "/api/stats/online?name=user>>>foo>>>online", "", "", 200},
		{"GET", "/api/stats/online/iplist?name=foo", "", "", 200},
		{"GET", "/api/query", "", "", 200},
		{"GET", "/api/online/users", "", "", 200},
		{"GET", "/api/traffic", "", "", 200},
		{"GET", "/api/traffic/hourly", "", "", 200},
		{"GET", "/api/traffic/hourly?hours=6", "", "", 200},
		{"GET", "/api/traffic/hourly?hours=abc", "", "", 400}, // hours 非法
		{"GET", "/api/traffic/hourly?from=bad", "", "", 400},  // from 时间格式无效
		{"GET", "/api/online", "", "", 200},
		{"GET", "/api/all", "", "", 200},
		// HandlerService。
		{"GET", "/api/inbounds/users?tag=vless-in", "", "", 200},
		{"GET", "/api/inbounds/users", "", "", 400}, // 缺 tag
		{"GET", "/api/inbounds/users/count?tag=vless-in", "", "", 200},
		{"POST", "/api/inbounds/users", `{"tag":"vless-in","email":"u1","id":"uuid-1"}`, testToken, 200},
		{"POST", "/api/inbounds/users", `{"tag":"vless-in","email":"u1","id":"uuid-1"}`, "", 403},      // 缺 token
		{"POST", "/api/inbounds/users", `{"tag":"vless-in","email":"u1","id":"uuid-1"}`, "wrong", 403}, // token 错误
		{"POST", "/api/inbounds/users", `{"email":"u1"}`, testToken, 400},                              // 缺必填字段
		{"DELETE", "/api/inbounds/users?tag=vless-in&email=u1", "", testToken, 200},
		{"DELETE", "/api/inbounds/users?tag=vless-in&email=u1", "", "", 403}, // 缺 token
		// RoutingService。
		{"GET", "/api/routing/rules", "", "", 200},
	}
	for _, c := range cases {
		var body *strings.Reader
		if c.body != "" {
			body = strings.NewReader(c.body)
		} else {
			body = strings.NewReader("")
		}
		req := httptest.NewRequest(c.method, c.path, body)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s %s: got %d want %d (body=%s)", c.method, c.path, rec.Code, c.want, rec.Body.String())
		}
		if rec.Code == 200 && !json.Valid(rec.Body.Bytes()) {
			t.Errorf("%s %s: invalid JSON: %s", c.method, c.path, rec.Body.String())
		}
	}
}
