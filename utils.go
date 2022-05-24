package rest_client

import (
	"runtime"
	"strconv"
	"strings"
)

type RestCallerInfo struct {
	FileName string
	FileLine string
	FuncName string
}

func pathCreate(basePath, path string) string {
	if len(basePath) == 0 {
		return path
	}
	if len(path) == 0 {
		return basePath
	}
	return basePath + "." + path
}

// callerFileInfo 获取调用方文件信息
func callerFileInfo(lastSkipMatch string, startLoop int, maxLoop int) *RestCallerInfo {
	fileName := ""
	fileLine := ""
	funcName := ""
	find := false
	for i := startLoop; i < maxLoop; i++ {
		pc, _fileName, _fileLine, ok := runtime.Caller(i)
		if pc == 0 {
			break
		}
		if strings.Contains(_fileName, lastSkipMatch) {
			find = true
		} else {
			if find {
				fileName = _fileName
				fileLine = strconv.Itoa(_fileLine)
				if ok {
					funcName = runtime.FuncForPC(pc).Name()
				}
				break
			}
		}
	}
	return &RestCallerInfo{
		FileName: fileName,
		FileLine: fileLine,
		FuncName: funcName,
	}
}
