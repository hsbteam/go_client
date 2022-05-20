package main

import (
	"context"
	"fmt"
	"net/http"
	"rest_client"
)

type RestDome1 struct {
	token string
}

const (
	ProductDetail = iota
	ProductAdd    = iota
)

//Config 接口列表配置
func (res *RestDome1) Config() (string, map[int]rest_client.RestBuild) {
	return "product", map[int]rest_client.RestBuild{
		ProductDetail: &rest_client.AppRestBuild{
			HttpMethod: http.MethodGet,
			//ParamMethod: http.MethodPost, //参数传递,默认等于  HttpMethod
			Path:   "/jp/product", //URL路径
			Method: "detail",      //接口名称
		},
		ProductAdd: &rest_client.AppRestBuild{
			HttpMethod: http.MethodPost,
			//ParamMethod: http.MethodGet, //参数传递,默认等于  HttpMethod
			Path:   "/jp/product", //URL路径
			Method: "add",         //接口名称
		},
	}
}
func (res *RestDome1) Token() string {
	return res.token
}

func main() {
	client := rest_client.NewHsbRestClient()
	//配置
	client.SetRestConfig("product", &rest_client.AppRestConfig{
		AppKey:    "hjx",
		AppSecret: "f4dea3417a2f52ae29a635be00537395",
		AppUrl:    "http://127.0.0.1:8080",
	})
	//使用
	data := <-client.NewApi(&RestDome1{
		token: "token_data",
	}).Do(context.Background(), ProductAdd, map[string]string{
		"id": "111",
	})
	if err := data.Err(); err != nil {
		fmt.Printf("error:%s", err)
		return
	}
	fmt.Printf("data:%s", data.JsonData(""))
}
