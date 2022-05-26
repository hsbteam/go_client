package main

import (
	"context"
	"fmt"
	"github.com/hsbteam/rest_client"
	"net/http"
)

type RestDome1 struct {
	token string
}

const (
	ProductDetail = iota
	ProductAdd    = iota
)

//Config 接口列表配置
func (res *RestDome1) Config(_ context.Context) (string, map[int]rest_client.RestBuild) {
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
func (res *RestDome1) Token(_ context.Context) string {
	return res.token
}

func main() {
	client := rest_client.NewHsbRestClient()
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
	}).Do(context.Background(), ProductDetail, map[string]string{
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
