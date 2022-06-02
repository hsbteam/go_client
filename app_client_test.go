package rest_client

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type testDome1 struct {
	token string
}

const (
	test1 = iota
	test2 = iota
)

func (res *testDome1) ConfigBuilds(_ context.Context) (map[int]RestBuild, error) {
	//动态构建配置发生错误时返回错误即可
	return map[int]RestBuild{
		test1: &AppRestBuild{
			HttpMethod: http.MethodGet,
			Path:       "/xxxxxxxxxx",
			Method:     "xxxxxxxxx",
			Timeout:    2 * time.Second,
		},
		test2: &AppRestBuild{
			HttpMethod: http.MethodPost,
			Path:       "/xxxxxxxxxxxx",
			Method:     "xxxxx",
			Timeout:    2 * time.Second,
		},
	}, nil
}
func (res *testDome1) ConfigName(_ context.Context) (string, error) {
	return "test111", nil
}

func (res *testDome1) Token(_ context.Context) (string, error) {
	return res.token, nil
}

func TestAppClient(t *testing.T) {
	client := NewRestClientManager()
	//配置
	client.SetRestConfig(&AppRestConfig{
		Name:      "test111",
		AppKey:    "dome1",
		AppSecret: "dome111111",
		AppUrl:    "http://8.8.8.8",
		EventCreate: func(_ context.Context) RestEvent {
			return NewAppRestEvent(
				func(_ string, _ string, _ int, _ map[string][]string, _ []byte, _ []byte, _ error) {
				})
		},
	})
	//使用
	data1 := (<-client.NewApi(&testDome1{
		token: "",
	}).Do(context.Background(), test1, map[string]string{
		"test": "111",
	})).JsonResult()
	if data1.Err() == nil {
		t.Error("test fail")
	}
	data2 := (<-client.NewApi(&testDome1{
		token: "",
	}).Do(context.Background(), test2, map[string]string{
		"test": "111",
	})).JsonResult()
	if data2.Err() == nil {
		t.Error("test fail")
	}
}
