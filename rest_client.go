package rest_client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/tidwall/gjson"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
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

type RestEvent interface {
	RequestStart(method, url string)                         //开始请求时回调
	RequestRead(p []byte)                                    //成功时读取请求数据回调
	ResponseHeader(HttpCode int, header map[string][]string) //成功返回HEADER时回调
	ResponseRead(p []byte)                                   //成功时读取请求内容
	FinishError(err error)                                   //结果为错误时回调 err
	FinishSuccess()                                          //结果正常时回调
}

type RestRequestReader struct {
	reader io.Reader
	event  RestEvent
}

func NewRestRequestReader(reader io.Reader, event RestEvent) *RestRequestReader {
	return &RestRequestReader{
		reader: reader,
		event:  event,
	}
}
func (read *RestRequestReader) Read(p []byte) (int, error) {
	if read.reader == nil {
		return 0, NewRestClientError("10", "request reader is empty")
	}
	//@todo 待测试...
	n, err := read.reader.Read(p)
	if read.event != nil && n > 0 {
		read.event.RequestRead(p[0:n])
	}
	return n, err
}

// RestBuild 执行请求
type RestBuild interface {
	BuildRequest(ctx context.Context, config *RestClient, param interface{}, callerInfo *RestCallerInfo) *RestResult
}

// RestCheckResult 检测返回结果是否正常
type RestCheckResult interface {
	CheckResult(res *RestResult) error
}

// RestConfig 执行请求
type RestConfig interface {
	GetName() string
}

// RestApi 接口定义
type RestApi interface {
	Config(ctx context.Context) (string, map[int]RestBuild)
}

// RestTokenApi 带TOKEN的接口定义
type RestTokenApi interface {
	RestApi
	Token(ctx context.Context) string
}

//RestClient 请求
type RestClient struct {
	Api       RestApi
	config    map[string]RestConfig
	transport *http.Transport
	event     RestEvent
}

func (client *RestClient) GetTransport() *http.Transport {
	return client.transport
}
func (client *RestClient) GetEvent() RestEvent {
	return client.event
}

//GetConfig 获取当前使用配置
func (client *RestClient) GetConfig(ctx context.Context) (RestConfig, error) {
	configName, _ := client.Api.Config(ctx)
	config, ok := client.config[configName]
	if !ok {
		return nil, NewRestClientError("1", "rest config is exits:"+configName)
	}
	return config, nil
}

func (client *RestClient) Do(ctx context.Context, key int, param interface{}) chan *RestResult {
	rc := make(chan *RestResult, 1)
	_, reqs := client.Api.Config(ctx)
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
	event    RestEvent
	valid    *validator.Validate
	build    RestBuild
	basePath string
	isRead   bool
	body     string
	err      error
	response *http.Response
}

//NewRestResultFromError 创建一个错误的请求结果
func NewRestResultFromError(err error, event RestEvent) *RestResult {
	result := &RestResult{
		event:    event,
		build:    nil,
		basePath: "",
		isRead:   true,
		body:     "",
		err:      err,
		response: nil,
	}
	if event != nil {
		event.FinishError(err)
	}
	return result
}

//NewRestResult 创建一个正常请求结果
func NewRestResult(build RestBuild, basePath string, response *http.Response, event RestEvent) *RestResult {
	result := &RestResult{
		event:    event,
		build:    build,
		basePath: basePath,
		isRead:   false,
		body:     "",
		err:      nil,
		response: response,
	}
	if event != nil {
		event.ResponseHeader(response.StatusCode, response.Header)
	}
	runtime.SetFinalizer(result, func(obj *RestResult) {
		if obj.event == nil {
			return
		}

		if obj.err != nil {
			obj.event.FinishSuccess()
		} else {
			obj.event.FinishError(obj.err)
		}
	})
	return result
}

//Read 读取接口
func (res *RestResult) Read(p []byte) (int, error) {
	if res.err != nil {
		return 0, res.err
	}
	n, err := res.response.Body.Read(p)
	//@todo 待测试...
	if n > 0 {
		res.event.ResponseRead(p[0:n])
	} else {
		if err != io.EOF {
			res.err = err
		}
	}
	return n, err
}

//Err 返回错误,无错误返回nil
func (res *RestResult) Err() error {
	return res.err
}

//ReadAll 返回整个BODY数据
func (res *RestResult) ReadAll() (string, error) {
	if !res.isRead {
		res.isRead = true
		body, err := ioutil.ReadAll(res)
		if err != nil {
			res.err = err
		}
		res.body = string(body)
	}
	return res.body, res.err
}

//parseJsonBody 检测返回结果是否正常
func (res *RestResult) parseJsonBody() error {
	if res.err != nil {
		return res.err
	}
	if check, ok := res.build.(RestCheckResult); ok {
		res.err = check.CheckResult(res)
		if res.err != nil {
			return res.err
		}
	}
	return nil
}

//JsonStructData 按指定路径把返回数据解析为结构
func (res *RestResult) JsonStructData(path string, structPtr interface{}, validPtr ...*validator.Validate) error {
	if res.err != nil {
		return res.err
	}
	if err := res.parseJsonBody(); err != nil {
		return err
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
	if err := res.parseJsonBody(); err != nil {
		return NewJsonResultFromError(err)
	}
	_path := res.basePath
	if len(path) > 0 {
		_path += "." + path
	}
	return NewJsonResult(gjson.Get(res.body, _path))
}

/////////////JSON结果数据//////////////////

// JsonResult JSON结果数据
type JsonResult struct {
	gjson.Result
	err error
}

// Err JSON结果是否错误
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
func (c *HsbRestClient) SetRestConfig(config RestConfig) *HsbRestClient {
	c.restConfig[config.GetName()] = config
	return c
}

//NewHsbRestClient 新建REST客户端
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
