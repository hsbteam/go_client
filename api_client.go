package rest_client

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"
)

//AppRestConfig 回收宝内部服务配置
type AppRestConfig struct {
	AppKey    string
	AppSecret string
	AppUrl    string
}

func (clf *AppRestConfig) GetConfig(key string) string {
	v := reflect.ValueOf(clf).Elem()
	return v.FieldByName(key).String()
}

//AppRestBuild 内部接口配置
type AppRestBuild struct {
	Timeout     time.Duration //指定接口超时时间,默认0,跟全局一致
	Path        string        //接口路径
	HttpMethod  string
	ParamMethod string
	Method      string
}

//BuildRequest 执行请求
func (clt *AppRestBuild) BuildRequest(_ context.Context, client *RestClient, param interface{}, _ *RestCallerInfo) *RestResult {
	config, err := client.GetConfig()
	if err != nil {
		return NewRestResultFromError(err, clt)
	}
	transport := client.GetTransport()
	headerTime := transport.ResponseHeaderTimeout

	jsonParam, err := json.Marshal(param)
	if err != nil {
		return NewRestResultFromError(err, clt)
	}

	apiUrl := config.GetConfig("AppUrl")
	appid := config.GetConfig("AppKey")
	keyConfig := config.GetConfig("AppSecret")

	reqParam := map[string]string{
		"app_key":   appid,
		"method":    clt.Method,
		"version":   "1.0",
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"content":   string(jsonParam),
	}

	if token, find := client.Api.(RestTokenApi); find {
		reqParam["token"] = token.Token()
	}

	var keys []string
	for k := range reqParam {
		keys = append(keys, k)
	}
	sort.Sort(sort.StringSlice(keys))
	data := url.Values{}
	for _, key := range keys {
		data.Set(key, reqParam[key])
	}
	reqData := data.Encode()

	dataSign := md5.Sum([]byte(reqData + keyConfig))
	reqParam["sign"] = fmt.Sprintf("%x", dataSign)

	pData := url.Values{}
	for key, val := range reqParam {
		pData.Set(key, val)
	}
	var ioRead io.Reader
	paramStr := pData.Encode()
	apiUrl += clt.Path
	if clt.ParamMethod == "" {
		clt.ParamMethod = clt.HttpMethod
	}
	if clt.ParamMethod == http.MethodGet {
		if strings.Index(apiUrl, "?") == -1 {
			apiUrl += "?" + paramStr
		} else {
			apiUrl += "&" + paramStr
		}
		ioRead = nil
	} else {
		ioRead = strings.NewReader(paramStr)
	}
	req, err := http.NewRequest(clt.HttpMethod, apiUrl, ioRead)
	if clt.HttpMethod == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return NewRestResultFromError(err, clt)
	}
	if clt.Timeout > 0 {
		transport.ResponseHeaderTimeout = clt.Timeout
	}
	httpClient := &http.Client{
		Transport: transport,
	}
	res, err := httpClient.Do(req)
	if clt.Timeout > 0 {
		transport.ResponseHeaderTimeout = headerTime
	}
	if err != nil {
		return NewRestResultFromError(err, clt)
	} else {
		return NewRestResult(clt, "response", res)
	}
}

func (clt *AppRestBuild) ParseResult(body string) error {
	code := gjson.Get(body, "result_response.code").String()
	if code != "200" {
		msg := gjson.Get(body, "result_response.msg").String()
		return NewRestClientError(code, "hsb server return fail:"+msg)
	}
	return nil
}
