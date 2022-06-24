package rest_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/tidwall/gjson"
	"reflect"
)

//JsonResult JSON结果集
type JsonResult struct {
	valid    *validator.Validate
	basePath string
	body     string
	err      error
}

// NewJsonResult 解析一个JSON字符串为JSON结果
//@param jsonBody JSON内容
//@param basePath 从某个节点获取,传入空字符串表示根节点获取
func NewJsonResult(jsonBody string, basePath string) *JsonResult {
	return &JsonResult{body: jsonBody, basePath: basePath}
}

// NewJsonResultFromError 创建一个错误JSON结果
func NewJsonResultFromError(err error) *JsonResult {
	return &JsonResult{err: err}
}

// Err JSON结果是否错误
func (res *JsonResult) Err() error {
	return res.err
}

//JsonValid JSON校验结构
type JsonValid struct {
	//外部定义校验结构
	valid *validator.Validate
	//上下文通过此结构透传,如果放到 GetStruct 跟 GetData 上参数太多,实用不方便
	Context context.Context
}

// GetStruct 从JSON中解析出结构并验证
func (res *JsonResult) GetStruct(path string, structPtr interface{}, jsonValid ...*JsonValid) error {
	if res.err != nil {
		return res.err
	}
	body := res.body
	var param string
	path = pathCreate(res.basePath, path)
	if len(path) == 0 {
		param = body
	} else {
		param = gjson.Get(body, path).String()
	}
	dec := json.NewDecoder(bytes.NewBuffer([]byte(param)))
	dec.UseNumber()
	err := dec.Decode(&structPtr)
	if err != nil {
		return err
	}
	//structPtr 非结构体不做校验
	val := reflect.ValueOf(structPtr)
	if val.Kind() == reflect.Ptr && !val.IsNil() {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}
	var valid *validator.Validate
	var ctx context.Context
	if jsonValid != nil && len(jsonValid) > 0 {
		valid = jsonValid[0].valid
		ctx = jsonValid[0].Context
	}
	if valid == nil {
		if res.valid == nil {
			res.valid = validator.New()
		}
		valid = res.valid
	}
	var vErr error
	if ctx == nil {
		vErr = valid.Struct(structPtr)
	} else {
		vErr = valid.StructCtx(ctx, structPtr)
	}
	if vErr != nil {
		return vErr
	}
	return nil
}

//JsonKey JSON获取KEY
type JsonKey struct {
	Path      string                                //获取路径
	ToType    func(result gjson.Result) interface{} //转换为指定类型
	Tag       string                                //校验用TAG
	JsonValid                                       //JSON校验结构
}

//GetData 从JSON中获取数据
//@param dataKey 传入string 表示不校验直接获取某节点数据,传空获取所有数据
func (res *JsonResult) GetData(dataKey interface{}) *JsonData {
	if res.err != nil {
		return NewJsonDataFromError(res.err)
	}
	var dKey *JsonKey
	if _dKey, ok := dataKey.(*JsonKey); ok {
		dKey = _dKey
	} else if path, ok := dataKey.(string); ok {
		dKey = &JsonKey{
			Path: path,
		}
	} else if dataKey == nil {
		dKey = &JsonKey{}
	} else {
		return NewJsonDataFromError(NewRestClientError("20", "dataKey type not support"))
	}
	body := res.body
	_path := pathCreate(res.basePath, dKey.Path)
	if len(_path) == 0 {
		return NewJsonData(&gjson.Result{
			Type: gjson.String,
			Str:  body,
		})
	}
	data := gjson.Get(body, _path)
	if len(dKey.Tag) > 0 {
		valid := dKey.valid
		if valid == nil {
			if res.valid == nil {
				res.valid = validator.New()
			}
			valid = res.valid
		}
		var val interface{}
		if dKey.ToType == nil {
			val = data.String()
		} else {
			val = dKey.ToType(data)
		}
		var err error
		if dKey.Context == nil {
			err = valid.Var(val, dKey.Tag)
		} else {
			err = valid.VarCtx(dKey.Context, val, dKey.Tag)
		}
		if err != nil {
			return NewJsonDataFromError(NewRestClientError("20", fmt.Sprintf("path:%s valid:%s", _path, err.Error())))
		}
	}
	return NewJsonData(&data)
}

/////////////JSON结果数据//////////////////

// JsonData JSON数据
type JsonData struct {
	*gjson.Result
	err error
}

// Err JSON数据是否错误,如校验失败时通过此函数返回错误详细
func (hand *JsonData) Err() error {
	return hand.err
}

// NewJsonData 创建一个正常JSON数据
func NewJsonData(result *gjson.Result) *JsonData {
	return &JsonData{result, nil}
}

// NewJsonDataFromError 创建一个错误JSON数据
func NewJsonDataFromError(err error) *JsonData {
	return &JsonData{err: err, Result: &gjson.Result{}}
}
