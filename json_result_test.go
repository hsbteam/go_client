package rest_client

import (
	"testing"
)

func TestNewJsonResult(t *testing.T) {
	read := NewJsonResult(`{"a":{"B":"cc"}}`, "a")
	if read.GetData("B").String() != "cc" {
		t.Error("json parse data error")
	}
	type Tmp struct {
		B string
	}
	var tmp Tmp
	if err := read.GetStruct("", &tmp); err != nil {
		t.Error(err)
	} else {
		if tmp.B != "cc" {
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
