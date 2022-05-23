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
	"sort"
	"strings"
	"time"
)

//AppRestConfig 回收宝内部服务配置
type AppRestConfig struct {
	Name        string
	AppKey      string
	AppSecret   string
	AppUrl      string
	EventCreate func(ctx context.Context) RestEvent
}

func (clf *AppRestConfig) GetName() string {
	return clf.Name
}

//AppRestBuild 内部接口配置
type AppRestBuild struct {
	Timeout     time.Duration //指定接口超时时间,默认0,跟全局一致
	Path        string        //接口路径
	HttpMethod  string
	ParamMethod string
	Method      string
}

func NewAppRestEvent(logger func(method string, url string, httpCode int, httpHeader map[string][]string, request []byte, response []byte, err error)) *AppRestEvent {
	return &AppRestEvent{
		logger: logger,
	}
}

//AppRestEvent 接口事件实现
type AppRestEvent struct {
	method     string
	url        string
	httpCode   int
	httpHeader map[string][]string
	request    []byte
	response   []byte
	logger     func(method string, url string, httpCode int, httpHeader map[string][]string, request []byte, response []byte, err error)
}

func (event *AppRestEvent) RequestStart(method, url string) {
	event.method = method
	event.url = url
}
func (event *AppRestEvent) RequestRead(data []byte) {
	event.request = append(event.request, data...)
}
func (event *AppRestEvent) ResponseHeader(httpCode int, httpHeader map[string][]string) {
	event.httpCode = httpCode
	event.httpHeader = httpHeader
}
func (event *AppRestEvent) ResponseRead(data []byte) {
	event.request = append(event.request, data...)
}
func (event *AppRestEvent) FinishError(err error) {
	if event.logger != nil {
		event.logger(event.method, event.url, event.httpCode, event.httpHeader, event.request, event.response, err)
	}
}
func (event *AppRestEvent) FinishSuccess() {
	if event.logger != nil {
		event.logger(event.method, event.url, event.httpCode, event.httpHeader, event.request, event.response, nil)
	}
}

//BuildRequest 执行请求
func (clt *AppRestBuild) BuildRequest(ctx context.Context, client *RestClient, param interface{}, _ *RestCallerInfo) *RestResult {
	tConfig, err := client.GetConfig(ctx)
	if err != nil {
		return NewRestResultFromError(err, &AppRestEvent{})
	}
	config, ok := tConfig.(*AppRestConfig)
	if !ok {
		return NewRestResultFromError(NewRestClientError("11", "build config is wrong"), &AppRestEvent{})
	}

	var event RestEvent
	if config.EventCreate != nil {
		event = config.EventCreate(ctx)
	} else {
		event = &AppRestEvent{}
	}

	transport := client.GetTransport()
	headerTime := transport.ResponseHeaderTimeout

	jsonParam, err := json.Marshal(param)
	if err != nil {
		return NewRestResultFromError(err, event)
	}

	apiUrl := config.AppUrl
	appid := config.AppKey
	keyConfig := config.AppSecret

	reqParam := map[string]string{
		"app_key":   appid,
		"method":    clt.Method,
		"version":   "1.0",
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"content":   string(jsonParam),
	}

	if token, find := client.Api.(RestTokenApi); find {
		reqParam["token"] = token.Token(ctx)
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
	event.RequestStart(clt.HttpMethod, apiUrl)
	req, err := http.NewRequest(clt.HttpMethod, apiUrl, NewRestRequestReader(ioRead, event))
	if clt.HttpMethod == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return NewRestResultFromError(err, event)
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
		return NewRestResultFromError(err, event)
	} else {
		return NewRestResult(clt, "response", res, event)
	}
}

func (clt *AppRestBuild) CheckResult(res *RestResult) error {
	body, err := res.ReadAll()
	if err != nil {
		return err
	}
	code := gjson.Get(body, "result_response.code").String()
	if code != "200" {
		msg := gjson.Get(body, "result_response.msg").String()
		return NewRestClientError(code, "hsb server return fail:"+msg)
	}
	return nil
}
