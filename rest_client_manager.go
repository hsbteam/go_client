package rest_client

import (
	"net"
	"net/http"
	"time"
)

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
			IdleConnTimeout:       15 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second, //默认到header的等待时间最长
		}
	} else {
		setTransport = transport[0]
	}
	return &RestClientManager{
		restConfig: make(map[string]RestConfig),
		transport:  setTransport,
	}
}
