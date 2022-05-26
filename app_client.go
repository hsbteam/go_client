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
	Timeout    time.Duration //指定接口超时时间,默认0,跟全局一致
	Path       string        //接口路径
	HttpMethod string
	Method     string
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
	event.response = append(event.response, data...)
}
func (event *AppRestEvent) ResponseFinish(err error) {
	if event.logger != nil {
		event.logger(event.method, event.url, event.httpCode, event.httpHeader, event.request, event.response, err)
	}
}
func (event *AppRestEvent) ResponseCheck(_ error) {}

//ParamSign 参数签名生成
func ParamSign(version, appKey, method, timestamp, content, appSecret string, token *string) string {
	reqParam := map[string]string{
		"app_key":   appKey,
		"method":    method,
		"version":   version,
		"timestamp": timestamp,
		"content":   content,
	}
	if token != nil {
		reqParam["token"] = *token
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
	dataSign := md5.Sum([]byte(reqData + appSecret))
	return fmt.Sprintf("%x", dataSign)
}

//BuildRequest 执行请求
func (clt *AppRestBuild) BuildRequest(ctx context.Context, client *RestClient, param interface{}, _ *RestCallerInfo) *RestResult {
	tConfig, err := client.GetConfig(ctx)
	if err != nil {
		return NewRestResultFromError(err, &RestEventNoop{})
	}
	config, ok := tConfig.(*AppRestConfig)
	if !ok {
		return NewRestResultFromError(NewRestClientError("11", "build config is wrong"), &RestEventNoop{})
	}

	var event RestEvent
	if config.EventCreate != nil {
		event = config.EventCreate(ctx)
	} else {
		event = &RestEventNoop{}
	}

	transport := client.GetTransport()
	headerTime := transport.ResponseHeaderTimeout
	apiUrl := config.AppUrl
	appid := config.AppKey
	keyConfig := config.AppSecret

	jsonParam, err := json.Marshal(param)
	if err != nil {
		return NewRestResultFromError(err, event)
	}

	var token *string
	if token_, find := client.Api.(RestTokenApi); find {
		tokenTmp := token_.Token(ctx)
		token = &tokenTmp
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	dataSign := ParamSign("1.0", appid, clt.Method, timestamp, string(jsonParam), keyConfig, token)
	reqParam := map[string]string{
		"app_key":   appid,
		"method":    clt.Method,
		"version":   "1.0",
		"timestamp": timestamp,
		"content":   string(jsonParam),
		"sign":      dataSign,
	}
	if token != nil {
		reqParam["token"] = *token
	}
	pData := url.Values{}
	for key, val := range reqParam {
		pData.Set(key, val)
	}
	paramStr := pData.Encode()
	apiUrl += clt.Path
	var ioRead io.Reader
	if clt.HttpMethod == http.MethodGet {
		if strings.Index(apiUrl, "?") == -1 {
			apiUrl += "?" + paramStr
		} else {
			apiUrl += "&" + paramStr
		}
		ioRead = nil
	} else {
		ioRead = NewRestRequestReader(strings.NewReader(paramStr), event)
	}
	event.RequestStart(clt.HttpMethod, apiUrl)
	var req *http.Request
	req, err = http.NewRequest(clt.HttpMethod, apiUrl, ioRead)
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
		return NewRestResult(clt, res, event)
	}
}

func (clt *AppRestBuild) CheckJsonResult(body string) error {
	code := gjson.Get(body, "result_response.code").String()
	if code != "200" {
		msg := gjson.Get(body, "result_response.msg").String()
		return NewRestClientError(code, "hsb server return fail:"+msg)
	}
	return nil
}
