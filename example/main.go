package main

import (
	"context"
	"fmt"
	"github.com/hsbteam/rest_client"
	"net/http"
	"time"
)

type RestDome1 struct {
	token string
}

const (
	ProductDetail = iota
	ProductAdd    = iota
)

func (res *RestDome1) ConfigBuilds(_ context.Context) (map[int]rest_client.RestBuild, error) {
	//动态构建配置发生错误时返回错误即可
	return map[int]rest_client.RestBuild{
		ProductDetail: &rest_client.AppRestBuild{
			HttpMethod: http.MethodGet,
			Path:       "/jp/product", //URL路径
			Method:     "detail",      //接口名称
			Timeout:    100 * time.Second,
		},
		ProductAdd: &rest_client.AppRestBuild{
			HttpMethod: http.MethodPost,
			Path:       "/jp/product", //URL路径
			Method:     "add",         //接口名称
			Timeout:    100 * time.Second,
		},
	}, nil
}
func (res *RestDome1) ConfigName(_ context.Context) (string, error) {
	//动态获取配置发生错误时返回错误即可
	return "product", nil
}

func (res *RestDome1) Token(_ context.Context) (string, error) {
	//动态获取TOKEN发生错误时返回错误即可
	return res.token, nil
}

func main() {
	client := rest_client.NewRestClientManager()
	//配置
	client.SetRestConfig(&rest_client.AppRestConfig{
		Name:      "product",
		AppKey:    "hjx",
		AppSecret: "f4dea3417a2f52ae29a635be00537395",
		AppUrl:    "http://127.0.0.1:8080",
		EventCreate: func(_ context.Context) rest_client.RestEvent {
			return rest_client.NewAppRestEvent(
				func(method string, url string, httpCode int, httpHeader map[string][]string, request []byte, response []byte, err error) {
					fmt.Printf("%s:%s [%d] \n", method, url, httpCode)
					fmt.Printf("request:%s \n", string(request))
					fmt.Printf("response:%s \n", string(response))
					if err != nil {
						fmt.Printf("error:%s \n", err.Error())
					}
				})
		},
	})
	//使用
	data := (<-client.NewApi(&RestDome1{
		token: "",
	}).Do(context.Background(), ProductAdd, map[string]string{
		"id": "111",
	})).JsonResult()
	if err := data.Err(); err != nil {
		fmt.Printf("error:%s", err)
		return
	}
	fmt.Printf("data:%s", data.GetData(""))
	//获取数据并校验
	//data.GetData(rest_client.JsonKey{
	//	Path: "data",
	//	ToType: func(result gjson.Result) interface{} {
	//		return result.String()
	//	},
	//	Tag: "glen:10",
	//	JsonValid: rest_client.JsonValid{
	//		Context: context.Background(),
	//	},
	//})
}
