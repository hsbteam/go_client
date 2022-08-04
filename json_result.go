package rest_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/tidwall/gjson"
	"reflect"
	"strings"
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

type JsonDataToType interface {
	JsonDataToType(field string, result interface{}) interface{}
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

	for i := 0; i < val.NumField(); i++ {
		tVal := val.Field(i)
		println(tVal.Kind())
		switch tVal.Kind() {
		case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
			if tVal.IsNil() {
				if jDat, ok := tVal.Interface().(JsonDataDefault); ok {
					defVal := jDat.JsonDataDefault()
					tVal.Set(reflect.ValueOf(defVal))
				}
			}
		case reflect.Struct:
			//结构赋值待定...
			//if reflect.PtrTo(tVal.Type()).Implements(reflect.TypeOf((*JsonDataDefault)(nil)).Elem()) {
			//
			//}
		}
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

	if tVal, ok := structPtr.(JsonDataToType); ok {
		retH := reflect.TypeOf(structPtr)
		if retH.Kind() == reflect.Ptr {
			retH = retH.Elem()
		}
		if val.Kind() == reflect.Struct {
			for i := 0; i < retH.NumField(); i++ {
				field := retH.Field(i)
				vTag := field.Tag.Get("validate")
				vVal := tVal.JsonDataToType(field.Name, val.Field(i).Interface())
				var vErr error
				if ctx == nil {
					vErr = valid.Var(vVal, vTag)
				} else {
					vErr = valid.VarCtx(ctx, vVal, vTag)
				}
				if vErr != nil {
					return NewRestClientError("20", fmt.Sprintf("path:%s field:%s tag:%s value:%v error:%s ", path, field.Name, vTag, vVal, vErr.Error()))
				}
			}
		}
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
	Path       string                                 //获取路径
	ToType     func(result *gjson.Result) interface{} //转换为指定类型
	Tag        string                                 //校验用TAG
	*JsonValid                                        //JSON校验结构
}

//GetData 从JSON中获取数据
//@param dataKey 传入string 表示不校验直接获取某节点数据,传空获取所有数据
func (res *JsonResult) GetData(key interface{}) *JsonData {
	if res.err != nil {
		return NewJsonDataFromError(res.err)
	}
	var dKey *JsonKey
	if _dKey, ok := key.(*JsonKey); ok {
		dKey = _dKey
	} else if path, ok := key.(string); ok {
		dKey = &JsonKey{
			Path: path,
		}
	} else if key == nil {
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
		var valid *validator.Validate
		if dKey.JsonValid != nil {
			valid = dKey.valid
		}
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
			val = dKey.ToType(&data)
		}
		var err error
		if dKey.JsonValid != nil && dKey.Context != nil {
			err = valid.VarCtx(dKey.Context, val, dKey.Tag)
		} else {
			err = valid.Var(val, dKey.Tag)
		}
		if err != nil {
			return NewJsonDataFromError(NewRestClientError("20", fmt.Sprintf("path:%s tag:%s value:%v error:%s ", _path, dKey.Tag, val, err.Error())))
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

func (hand *JsonData) UnmarshalJSON(data []byte) error {
	if hand == nil {
		return nil
	}
	jDat := JsonData{
		&gjson.Result{
			Type: gjson.String,
			Str:  strings.Trim(string(data), "\""),
		}, nil,
	}
	*hand = jDat
	return nil
}

type JsonDataDefault interface {
	JsonDataDefault() interface{}
	//	JsonDataStructDefault()
}

func (hand *JsonData) JsonDataDefault() interface{} {
	return NewJsonData(&gjson.Result{
		Type: gjson.Null,
	})
}

//func (hand JsonData) JsonDataStructDefault() {
//	if hand.Result == nil {
//		hand.Result = &gjson.Result{
//			Type: gjson.Null,
//		}
//	}
//}

// NewJsonData 创建一个正常JSON数据
func NewJsonData(result *gjson.Result) *JsonData {
	return &JsonData{result, nil}
}

// NewJsonDataFromError 创建一个错误JSON数据
func NewJsonDataFromError(err error) *JsonData {
	return &JsonData{err: err, Result: &gjson.Result{}}
}
