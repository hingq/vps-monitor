package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"

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

func TestRoutesSmoke(t *testing.T) {
	srv := New(stats.NewService(fakeClient{}))
	mux := srv.Routes()

	cases := []struct {
		path string
		want int
	}{
		{"/api/sysstats", 200},
		{"/api/stats?name=user>>>foo>>>traffic>>>uplink", 200},
		{"/api/stats", 400}, // 缺 name
		{"/api/stats/online?name=user>>>foo>>>online", 200},
		{"/api/stats/online/iplist?name=foo", 200},
		{"/api/query", 200},
		{"/api/online/users", 200},
		{"/api/traffic", 200},
		{"/api/online", 200},
		{"/api/all", 200},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: got %d want %d (body=%s)", c.path, rec.Code, c.want, rec.Body.String())
		}
		if c.want == 200 && !json.Valid(rec.Body.Bytes()) {
			t.Errorf("%s: invalid JSON: %s", c.path, rec.Body.String())
		}
	}
}
