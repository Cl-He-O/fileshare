package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

var B64 = base64.RawURLEncoding

func content_handler(name string, content []byte) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(content))
	}
}

func respond_text(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	w.Write([]byte(text))
}

func respond_message(w http.ResponseWriter, msg string, success bool) {
	response := make(map[string]any)

	if !success {
		response["message"] = msg
	} else {
		response["success"] = true
	}

	resp, _ := json.Marshal(response)
	w.Write(resp)
}
