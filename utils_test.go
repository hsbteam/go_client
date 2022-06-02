package rest_client

import (
	"strings"
	"testing"
)

func TestPathCreate(t *testing.T) {
	if pathCreate("", "a") != "a" {
		t.Error("'' and 'a' != a")
	}
	if pathCreate("a", "") != "a" {
		t.Error("'a' and '' != a")
	}
	if pathCreate("a", "a") != "a.a" {
		t.Error("'a' and 'a' != a.a")
	}
}

func TestCallerFileInfo(t *testing.T) {
	data := callerFileInfo("utils.go", 0, 12)
	if !strings.Contains(data.FuncName, "TestCallerFileInfo") {
		t.Error("not find self function name")
	}
	if !strings.Contains(data.FileName, "utils_test.go") {
		t.Error("not find self file name")
	}
}
