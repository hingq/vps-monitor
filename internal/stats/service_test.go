package stats

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
)

// fakeClient 是可配置的 command.StatsServiceClient 测试替身。
type fakeClient struct {
	sysStats   *command.SysStatsResponse
	getStats   *command.GetStatsResponse
	online     *command.GetStatsResponse
	query      *command.QueryStatsResponse
	ipList     *command.GetStatsOnlineIpListResponse
	users      *command.GetAllOnlineUsersResponse
	err        error // 非空时所有方法返回该错误
	ipListErrs map[string]error
}

func (f fakeClient) GetStats(_ context.Context, in *command.GetStatsRequest, _ ...grpc.CallOption) (*command.GetStatsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.getStats, nil
}
func (f fakeClient) GetStatsOnline(_ context.Context, _ *command.GetStatsRequest, _ ...grpc.CallOption) (*command.GetStatsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.online, nil
}
func (f fakeClient) QueryStats(_ context.Context, _ *command.QueryStatsRequest, _ ...grpc.CallOption) (*command.QueryStatsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.query, nil
}
func (f fakeClient) GetSysStats(_ context.Context, _ *command.SysStatsRequest, _ ...grpc.CallOption) (*command.SysStatsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.sysStats, nil
}
func (f fakeClient) GetStatsOnlineIpList(_ context.Context, in *command.GetStatsRequest, _ ...grpc.CallOption) (*command.GetStatsOnlineIpListResponse, error) {
	if err, ok := f.ipListErrs[in.Name]; ok {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.ipList, nil
}
func (f fakeClient) GetAllOnlineUsers(_ context.Context, _ *command.GetAllOnlineUsersRequest, _ ...grpc.CallOption) (*command.GetAllOnlineUsersResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.users, nil
}

func TestSysStats(t *testing.T) {
	svc := NewService(fakeClient{sysStats: &command.SysStatsResponse{
		Uptime: 42, NumGoroutine: 7, NumGC: 2, Alloc: 1024, TotalAlloc: 2048, Sys: 1024 * 1024,
	}})
	got, err := svc.SysStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.UptimeSeconds != 42 || got.NumGoroutine != 7 {
		t.Errorf("scalar fields wrong: %+v", got)
	}
	if got.AllocHuman != "1.00 KB" || got.SysHuman != "1.00 MB" {
		t.Errorf("human fields wrong: alloc=%q sys=%q", got.AllocHuman, got.SysHuman)
	}
}

func TestGetStat(t *testing.T) {
	svc := NewService(fakeClient{getStats: &command.GetStatsResponse{
		Stat: &command.Stat{Name: "user>>>foo>>>traffic>>>uplink", Value: 2048},
	}})
	got, err := svc.GetStat(context.Background(), "user>>>foo>>>traffic>>>uplink", false)
	if err != nil {
		t.Fatal(err)
	}
	want := &StatJSON{Name: "user>>>foo>>>traffic>>>uplink", Value: 2048, ValueHuman: "2.00 KB"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestGetStatNilStat(t *testing.T) {
	// gRPC 可能返回 nil Stat，应安全降级而非 panic。
	svc := NewService(fakeClient{getStats: &command.GetStatsResponse{}})
	got, err := svc.GetStat(context.Background(), "missing", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != 0 || got.ValueHuman != "0 B" {
		t.Errorf("nil stat fallback wrong: %+v", got)
	}
}

func TestGetStatOnline(t *testing.T) {
	svc := NewService(fakeClient{online: &command.GetStatsResponse{
		Stat: &command.Stat{Name: "user>>>foo>>>online", Value: 3},
	}})
	got, err := svc.GetStatOnline(context.Background(), "user>>>foo>>>online", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "user>>>foo>>>online" || got.Value != 3 {
		t.Errorf("got %+v", got)
	}
}

func TestOnlineIPListSorted(t *testing.T) {
	svc := NewService(fakeClient{ipList: &command.GetStatsOnlineIpListResponse{
		Ips: map[string]int64{"9.9.9.9": 100, "1.1.1.1": 200, "5.5.5.5": 150},
	}})
	got, err := svc.OnlineIPList(context.Background(), "foo")
	if err != nil {
		t.Fatal(err)
	}
	gotIPs := []string{got[0].IP, got[1].IP, got[2].IP}
	want := []string{"1.1.1.1", "5.5.5.5", "9.9.9.9"}
	if !reflect.DeepEqual(gotIPs, want) {
		t.Errorf("ip order = %v, want %v", gotIPs, want)
	}
	if got[0].LastSeenUnix != 200 || got[0].LastSeen == "" {
		t.Errorf("timestamp fields wrong: %+v", got[0])
	}
}

func TestQueryRaw(t *testing.T) {
	svc := NewService(fakeClient{query: &command.QueryStatsResponse{Stat: []*command.Stat{
		{Name: "a", Value: 1024},
		{Name: "b", Value: 0},
	}}})
	got, err := svc.QueryRaw(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ValueHuman != "1.00 KB" || got[1].ValueHuman != "0 B" {
		t.Errorf("got %+v", got)
	}
}

func TestAllOnlineUsersSorted(t *testing.T) {
	svc := NewService(fakeClient{users: &command.GetAllOnlineUsersResponse{
		Users: []string{"charlie", "alice", "bob"},
	}})
	got, err := svc.AllOnlineUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alice", "bob", "charlie"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTrafficAggregation(t *testing.T) {
	svc := NewService(fakeClient{query: &command.QueryStatsResponse{Stat: []*command.Stat{
		{Name: "user>>>foo>>>traffic>>>uplink", Value: 1024},
		{Name: "user>>>foo>>>traffic>>>downlink", Value: 2048},
		{Name: "inbound>>>api>>>traffic>>>uplink", Value: 512},
		{Name: "malformed>>>name", Value: 999}, // 非 4 段，应被忽略
	}}})
	got, err := svc.Traffic(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 分类顺序固定 user -> inbound（outbound 无数据被跳过）。
	if len(got) != 2 || got[0].Category != "user" || got[1].Category != "inbound" {
		t.Fatalf("category order wrong: %+v", got)
	}
	u := got[0].Entries[0]
	if u.Name != "foo" || u.Uplink != 1024 || u.Downlink != 2048 || u.Total != 3072 {
		t.Errorf("user entry wrong: %+v", u)
	}
	if u.TotalHuman != "3.00 KB" {
		t.Errorf("total human = %q", u.TotalHuman)
	}
}

func TestOnlineUsersAggregation(t *testing.T) {
	svc := NewService(fakeClient{
		users:  &command.GetAllOnlineUsersResponse{Users: []string{"foo"}},
		ipList: &command.GetStatsOnlineIpListResponse{Ips: map[string]int64{"1.2.3.4": 1718700000}},
	})
	got, err := svc.OnlineUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].User != "foo" || len(got[0].IPs) != 1 || got[0].IPs[0].IP != "1.2.3.4" {
		t.Errorf("got %+v", got)
	}
}

func TestOnlineUsersIPErrorTolerated(t *testing.T) {
	// 单个用户 IP 查询失败不应中断整体，其 IPs 置空。
	svc := NewService(fakeClient{
		users:      &command.GetAllOnlineUsersResponse{Users: []string{"foo"}},
		ipListErrs: map[string]error{"foo": errors.New("boom")},
	})
	got, err := svc.OnlineUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].User != "foo" || len(got[0].IPs) != 0 {
		t.Errorf("expected empty IPs on error, got %+v", got)
	}
}

func TestServiceErrorPropagation(t *testing.T) {
	svc := NewService(fakeClient{err: errors.New("rpc down")})
	if _, err := svc.SysStats(context.Background()); err == nil {
		t.Error("SysStats: expected error")
	}
	if _, err := svc.Traffic(context.Background()); err == nil {
		t.Error("Traffic: expected error")
	}
	if _, err := svc.AllOnlineUsers(context.Background()); err == nil {
		t.Error("AllOnlineUsers: expected error")
	}
}
