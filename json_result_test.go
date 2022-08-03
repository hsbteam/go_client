package rest_client

import (
	"fmt"
	"github.com/tidwall/gjson"
	"strconv"
	"testing"
)

func TestNewJsonResult(t *testing.T) {
	b1 := "11"
	read := NewJsonResult(fmt.Sprintf(`{"a":{"B":"%s"}}`, b1), "a")
	if read.GetData("B").String() != b1 {
		t.Error("json parse data error")
	}
	tmpB1, _ := strconv.Atoi(b1)
	if read.GetData(&JsonKey{
		Path: "B",
		ToType: func(result *gjson.Result) interface{} {
			return result.Int()
		},
		Tag: "gte=0,lte=130",
	}).Int() != int64(tmpB1) {
		t.Error("json parse data error")
	}
	type Tmp struct {
		B string
	}
	var tmp Tmp
	if err := read.GetStruct("", &tmp); err != nil {
		t.Error(err)
	} else {
		if tmp.B != b1 {
			t.Error("json parse data error")
		}
	}
}

type tmpStr struct {
	B  *JsonData `validate:"required,email"`
	B1 *JsonData `validate:"gte=0,lte=130"`
}

func (receiver *tmpStr) JsonDataToType(field string, result *gjson.Result) interface{} {
	switch field {
	case "B":
		return result.String()
	case "B1":
		return result.Int()
	}
	return result
}

func TestNewJsonResultList(t *testing.T) {
	b := "sss@qq.com"
	b1 := "1111"
	read := NewJsonResult(fmt.Sprintf(`{"a":{"B":"%s","B1":"%s"}}`, b, b1), "a")
	var tmp tmpStr
	if err := read.GetStruct("", &tmp); err != nil {
		t.Error(err)
	} else {
		kk := tmp.B.String()
		if kk != b {
			t.Error("json parse data error")
		}
		kk1 := tmp.B1.String()
		if kk1 != b1 {
			t.Error("json parse data error")
		}
	}
}

func TestNewJsonResultErr(t *testing.T) {
	err := NewAppClientError("1", "2", "err")
	read := NewJsonResultFromError(err)
	if read.Err() == nil {
		t.Error("json parse error wrong")
	}
	if read.GetData("any data").Err() == nil {
		t.Error("json parse error wrong")
	}
}
