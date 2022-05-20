package rest_client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// RestClientError  错误信息
type RestClientError struct {
	Msg  string
	Code string
}

func (err *RestClientError) Error() string {
	return fmt.Sprintf("Code:%s Message:%s", err.Msg, err.Code)
}

// NewRestClientError  错误创建
func NewRestClientError(code string, msg string) *RestClientError {
	return &RestClientError{
		Code: code,
		Msg:  msg,
	}
}

// RestBuild 执行请求
type RestBuild interface {
	BuildRequest(ctx context.Context, config *RestClient, param interface{}, callerInfo *RestCallerInfo) *RestResult
	ParseResult(body string) error
}

// RestConfig 执行请求
type RestConfig interface {
	GetConfig(key string) string
}

// RestApi 详细定义接口
type RestApi interface {
	Config() (string, map[int]RestBuild)
}

type RestTokenApi interface {
	RestApi
	Token() string
}

//RestClient 请求
type RestClient struct {
	Api       RestApi
	config    map[string]RestConfig
	transport *http.Transport
}

func (client *RestClient) GetTransport() *http.Transport {
	return client.transport
}

//GetConfig 获取当前使用配置
func (client *RestClient) GetConfig() (RestConfig, error) {
	configName, _ := client.Api.Config()
	config, ok := client.config[configName]
	if !ok {
		return nil, NewRestClientError("1", "rest config is exits:"+configName)
	}
	return config, nil
}

func (client *RestClient) Do(ctx context.Context, key int, param interface{}) chan *RestResult {
	rc := make(chan *RestResult, 1)
	_, reqs := client.Api.Config()
	build, find := reqs[key]
	if !find {
		rc <- NewRestResultFromError(NewRestClientError("2", "not find rest api"), nil)
		close(rc)
	} else {
		caller := callerFileInfo("hsb_client/rest_client.go", 1, 15)
		go func() {
			res := build.BuildRequest(ctx, client, param, caller)
			rc <- res
			close(rc)
		}()
	}
	return rc
}

//RestResult 请求接口后返回数据结构
type RestResult struct {
	valid    *validator.Validate
	build    RestBuild
	basePath string
	isRead   bool
	body     string
	err      error
	Response *http.Response
}

//NewRestResultFromError 创建一个错误的请求结果
func NewRestResultFromError(err error, build RestBuild) *RestResult {
	return &RestResult{
		build:    build,
		basePath: "",
		isRead:   true,
		body:     "",
		err:      err,
		Response: nil,
	}
}

//NewRestResult 创建一个正常请求结果
func NewRestResult(build RestBuild, basePath string, response *http.Response) *RestResult {
	return &RestResult{
		build:    build,
		basePath: basePath,
		isRead:   false,
		body:     "",
		err:      nil,
		Response: response,
	}
}

//IsJson 判断返回内容是否是完整JSON字符
func (res *RestResult) IsJson() bool {
	return json.Valid([]byte(res.GetAllBody()))
}

func (res *RestResult) Err() error {
	return res.err
}

//GetAllBody 返回整个BODY数据
func (res *RestResult) GetAllBody() string {
	if !res.isRead {
		res.isRead = true
		body, err := ioutil.ReadAll(res.Response.Body)
		if err != nil {
			res.err = err
			return ""
		}
		res.body = string(body)
		if res.build == nil {
			res.err = NewRestClientError("9", "build is empty")
			return ""
		}
		res.err = res.build.ParseResult(res.body)
	}
	return res.body
}

//JsonStructData 按指定路径把返回数据解析为结构
func (res *RestResult) JsonStructData(path string, structPtr interface{}, validPtr ...*validator.Validate) error {
	if res.err != nil {
		return res.err
	}
	body := res.GetAllBody()
	if !res.IsJson() {
		return NewRestClientError("3", "service output not json:"+body)
	}
	var param string
	_path := res.basePath
	if len(path) > 0 {
		_path += "." + path
	}
	param = gjson.Get(res.body, _path).String()
	err := json.Unmarshal([]byte(param), structPtr)
	if err != nil {
		return err
	}
	if len(validPtr) > 0 {
		errs := validPtr[0].Struct(structPtr)
		if errs != nil {
			return errs
		}
	} else {
		if res.valid == nil {
			res.valid = validator.New()
		}
		errs := res.valid.Struct(structPtr)
		if errs != nil {
			return errs
		}
	}
	return nil
}

//JsonData 返回JSON中指定路径的数据
func (res *RestResult) JsonData(path string) *JsonResult {
	if res.err != nil {
		return NewJsonResultFromError(res.err)
	}
	body := res.GetAllBody()
	if !res.IsJson() {
		return NewJsonResultFromError(NewRestClientError("3", "service output not json:"+body))
	}
	_path := res.basePath
	if len(path) > 0 {
		_path += "." + path
	}
	return NewJsonResult(gjson.Get(body, _path))
}

/////////////JSON数据//////////////////

// JsonResult JSON结果数据
type JsonResult struct {
	gjson.Result
	err error
}

func (hand *JsonResult) Err() error {
	return hand.err
}

// NewJsonResult 创建一个正常JSON结果
func NewJsonResult(result gjson.Result) *JsonResult {
	return &JsonResult{result, nil}
}

// NewJsonResultFromError 创建一个错误JSON结果
func NewJsonResultFromError(err error) *JsonResult {
	return &JsonResult{err: err, Result: gjson.Result{}}
}

/////////////// 对外接口部分//////////////////

type HsbRestClient struct {
	restConfig map[string]RestConfig
	transport  *http.Transport
}

func (c *HsbRestClient) NewApi(api RestApi) *RestClient {
	rest := &RestClient{
		Api:       api,
		config:    c.restConfig,
		transport: c.transport,
	}
	return rest
}

//SetRestConfig 设置外部接口配置
func (c *HsbRestClient) SetRestConfig(name string, config RestConfig) *HsbRestClient {
	c.restConfig[name] = config
	return c
}

func NewHsbRestClient(transport ...*http.Transport) *HsbRestClient {
	var setTransport *http.Transport
	if transport == nil {
		setTransport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 300 * time.Second,
			}).DialContext,
			MaxIdleConns:        120,
			MaxIdleConnsPerHost: 12,
			IdleConnTimeout:     8 * time.Second,
		}
	} else {
		setTransport = transport[0]
	}
	return &HsbRestClient{
		restConfig: make(map[string]RestConfig),
		transport:  setTransport,
	}
}
