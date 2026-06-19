package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/xtls/xray-core/app/router/command"
	"google.golang.org/grpc"
)

type fakeClient struct {
	rules *command.ListRuleResponse
	err   error
}

func (f fakeClient) ListRule(_ context.Context, _ *command.ListRuleRequest, _ ...grpc.CallOption) (*command.ListRuleResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rules, nil
}

func TestListRules(t *testing.T) {
	svc := NewService(fakeClient{rules: &command.ListRuleResponse{Rules: []*command.ListRuleItem{
		{Tag: "direct", RuleTag: "r1"},
		{Tag: "block", RuleTag: "r2"},
	}}})

	got, err := svc.ListRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Tag != "direct" || got[1].RuleTag != "r2" {
		t.Errorf("规则解析错误: %+v", got)
	}
}

func TestListRulesError(t *testing.T) {
	svc := NewService(fakeClient{err: errors.New("boom")})
	if _, err := svc.ListRules(context.Background()); err == nil {
		t.Error("应返回错误")
	}
}
