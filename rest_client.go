package rest_client

import (
	"context"
	"fmt"
	"io"
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
	return fmt.Sprintf("%s [%s]", err.Msg, err.Code)
}

// NewRestClientError  错误创建
func NewRestClientError(code string, msg string) *RestClientError {
	return &RestClientError{
		Code: code,
		Msg:  msg,
	}
}

// RestEvent  事件接口,用于暴露对外请求的时的信息
type RestEvent interface {
	RequestStart(method, url string)                         //开始请求时回调
	RequestRead(p []byte)                                    //成功时读取请求数据回调
	ResponseHeader(HttpCode int, header map[string][]string) //成功返回HEADER时回调
	ResponseRead(p []byte)                                   //成功时读取请求内容
	ResponseFinish(err error)                                //内容读取完时回调,不存在错误时err为nil
	ResponseCheck(err error)                                 //检测返回内容是否正常,正常时err为nil
}

// RestEventNoop  默认事件处理
type RestEventNoop struct{}

func (event *RestEventNoop) RequestStart(_, _ string)                    {}
func (event *RestEventNoop) RequestRead(_ []byte)                        {}
func (event *RestEventNoop) ResponseHeader(_ int, _ map[string][]string) {}
func (event *RestEventNoop) ResponseRead(_ []byte)                       {}
func (event *RestEventNoop) ResponseFinish(_ error)                      {}
func (event *RestEventNoop) ResponseCheck(_ error)                       {}

//RestRequestReader 对请求io.Reader封装,用于读取内容时事件回调
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

type RestJsonResult interface {
	CheckJsonResult(res string) error
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
}

func (client *RestClient) GetTransport() *http.Transport {
	return client.transport
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
		caller := callerFileInfo("rest_client/rest_client.go", 1, 15)
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
	event          RestEvent
	build          RestBuild
	response       *http.Response
	body           string
	bodyReadOffset int
	err            error
}

//NewRestResultFromError 创建一个错误的请求结果
func NewRestResultFromError(err error, event RestEvent) *RestResult {
	result := &RestResult{
		event:          event,
		build:          nil,
		bodyReadOffset: -1,
		body:           "",
		err:            err,
		response:       nil,
	}
	if event != nil {
		event.ResponseFinish(err)
	}
	return result
}

//NewRestResult 创建一个正常请求结果
func NewRestResult(build RestBuild, response *http.Response, event RestEvent) *RestResult {
	result := &RestResult{
		event:          event,
		build:          build,
		bodyReadOffset: -1,
		body:           "",
		err:            nil,
		response:       response,
	}
	if event != nil {
		event.ResponseHeader(response.StatusCode, response.Header)
	}
	return result
}

//NewRestBodyResult 创建外部已经读取Response BODY的请求结果
func NewRestBodyResult(build RestBuild, body string, response *http.Response, event RestEvent) *RestResult {
	result := &RestResult{
		event:          event,
		build:          build,
		bodyReadOffset: 0,
		body:           body,
		err:            nil,
		response:       response,
	}
	if event != nil {
		event.ResponseHeader(response.StatusCode, response.Header)
		event.ResponseFinish(nil)
	}
	return result
}

//Read 读取接口
func (res *RestResult) Read(p []byte) (int, error) {
	if res.err != nil {
		return 0, res.err
	}
	if res.bodyReadOffset >= 0 {
		bDat := []byte(res.body)
		pLen := len(p)
		sLen := len(bDat[res.bodyReadOffset:])
		if sLen == 0 {
			return 0, io.EOF
		}
		if sLen > pLen {
			tmp := bDat[res.bodyReadOffset : res.bodyReadOffset+pLen]
			copy(p[0:pLen], tmp)
			res.bodyReadOffset += pLen
			return pLen, nil
		} else {
			tmp := bDat[res.bodyReadOffset : res.bodyReadOffset+sLen]
			copy(p[0:sLen], tmp)
			res.bodyReadOffset += sLen
			return sLen, io.EOF
		}
	} else {
		n, err := res.response.Body.Read(p)
		if n > 0 {
			res.event.ResponseRead(p[0:n])
		}
		if err == io.EOF {
			if res.event != nil {
				res.event.ResponseFinish(nil)
			}
		} else {
			res.err = err
			if res.event != nil {
				res.event.ResponseFinish(err)
			}
		}
		return n, err
	}
}

//Err 返回错误,无错误返回nil
func (res *RestResult) Err() error {
	return res.err
}

func (res *RestResult) JsonResult(path ...string) *JsonResult {
	defer func() {
		if res.event != nil {
			res.event.ResponseCheck(res.err)
		}
	}()
	if res.err != nil {
		return NewJsonResultFromError(res.err)
	}
	body, err := ioutil.ReadAll(res)
	if err != nil {
		return NewJsonResultFromError(res.err)
	}
	bodyStr := string(body)
	if check, ok := res.build.(RestJsonResult); ok {
		res.err = check.CheckJsonResult(bodyStr)
		if res.err != nil {
			return NewJsonResultFromError(res.err)
		}
	}
	basePath := ""
	if path != nil {
		basePath = path[0]
	}
	return NewJsonResult(bodyStr, basePath)
}

/////////////// 对外接口部分//////////////////

type RestClientManager struct {
	restConfig map[string]RestConfig
	transport  *http.Transport
}

func (c *RestClientManager) NewApi(api RestApi) *RestClient {
	rest := &RestClient{
		Api:       api,
		config:    c.restConfig,
		transport: c.transport,
	}
	return rest
}

//SetRestConfig 设置外部接口配置
func (c *RestClientManager) SetRestConfig(config RestConfig) *RestClientManager {
	c.restConfig[config.GetName()] = config
	return c
}

//NewRestClientManager 新建REST客户端
func NewRestClientManager(transport ...*http.Transport) *RestClientManager {
	var setTransport *http.Transport
	if transport == nil {
		setTransport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 300 * time.Second,
			}).DialContext,
			MaxIdleConns:          120,
			MaxIdleConnsPerHost:   12,
			IdleConnTimeout:       8 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second, //默认到header的等待时间最长
		}
	} else {
		setTransport = transport[0]
	}
	return &RestClientManager{
		restConfig: make(map[string]RestConfig),
		transport:  setTransport,
	}
}
