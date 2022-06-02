package rest_client

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestRestRequestReader(t *testing.T) {
	str := strings.NewReader("ssss")
	read := NewRestRequestReader(str, NewRestEventNoop())
	data, err := ioutil.ReadAll(read)
	if err != nil {
		t.Error(err)
	}
	if string(data) != "ssss" {
		t.Error("request reader read data error")
	}
}

func TestRestResult(t *testing.T) {
	err := NewAppClientError("1", "2", "err")
	bb := NewRestResultFromError(err, NewRestEventNoop())
	if bb.Err() != err {
		t.Error("error wrong")
	}
}

func TestRestBodyResult(t *testing.T) {
	bb1 := NewRestBodyResult(nil, `{"A":"11"}`, nil, NewRestEventNoop())
	if bb1.Err() != nil {
		t.Error("body is not error")
	}
	jb := bb1.JsonResult("")
	if jb.Err() != nil {
		t.Error("json not error")
	}
	if jb.GetData("A").String() != "11" {
		t.Error("json parse error")
	}
	type A struct {
		A string
	}
	var a A
	if err1 := jb.GetStruct("", &a); err1 != nil {
		t.Error(err1)
	}
	if a.A != "11" {
		t.Error("json parse struct error")
	}
}

func TestRestRespResult(t *testing.T) {
	body := ioutil.NopCloser(bytes.NewReader([]byte(`{"A":"11"}`)))
	response := &http.Response{
		Status:           "200",
		StatusCode:       0,
		Proto:            "http",
		ProtoMajor:       0,
		ProtoMinor:       0,
		Header:           nil,
		Body:             body,
		ContentLength:    12,
		TransferEncoding: nil,
		Close:            false,
		Uncompressed:     false,
		Trailer:          nil,
		Request:          nil,
		TLS:              nil,
	}
	bb1 := NewRestResult(nil, response, NewRestEventNoop())
	if bb1.Err() != nil {
		t.Error("body is not error")
	}
	jb := bb1.JsonResult("")
	if jb.Err() != nil {
		t.Error("json not error")
	}
	if jb.GetData("A").String() != "11" {
		t.Error("json parse error")
	}
	type A struct {
		A string
	}
	var a A
	if err1 := jb.GetStruct("", &a); err1 != nil {
		t.Error(err1)
	}
	if a.A != "11" {
		t.Error("json parse struct error")
	}
}
