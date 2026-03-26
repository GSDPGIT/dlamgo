package main

import (
	"encoding/json"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, payload APIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func ok(data interface{}) APIResponse {
	return APIResponse{
		Code: 0,
		Msg:  "操作成功",
		TS:   time.Now().UnixMilli(),
		Data: data,
	}
}

func okNoData() APIResponse {
	return APIResponse{
		Code: 0,
		Msg:  "操作成功",
		TS:   time.Now().UnixMilli(),
	}
}

func errResp(code int, msg string) APIResponse {
	return APIResponse{
		Code: code,
		Msg:  msg,
		TS:   time.Now().UnixMilli(),
	}
}
