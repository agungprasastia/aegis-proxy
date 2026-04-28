package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
)

var (
	ErrNoAccounts      = "No available accounts for this model"
	ErrModelNotFound   = "Model not found"
	ErrRateLimited     = "Rate limit exceeded"
	ErrInternal        = "Internal server error"
	ErrInvalidRequest  = "Invalid request"
)

type OpenAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func writeOpenAIError(w http.ResponseWriter, msg string, errType string, code string, httpStatus int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	
	errResp := OpenAIError{}
	errResp.Error.Message = msg
	errResp.Error.Type = errType
	errResp.Error.Code = code
	
	json.NewEncoder(w).Encode(errResp)
}

func writeAnthropicError(w http.ResponseWriter, msg string, errType string, httpStatus int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	
	errResp := AnthropicError{
		Type:    errType,
		Message: msg,
	}
	
	json.NewEncoder(w).Encode(errResp)
}

func writeError(w http.ResponseWriter, r *http.Request, msg string, httpStatus int) {
	contentType := r.Header.Get("Content-Type")
	anthropicVersion := r.Header.Get("anthropic-version")
	
	if strings.Contains(contentType, "application/json") && anthropicVersion != "" {
		writeAnthropicError(w, msg, "api_error", httpStatus)
		return
	}
	
	if strings.Contains(r.URL.Path, "/v1/messages") {
		writeAnthropicError(w, msg, "api_error", httpStatus)
		return
	}
	
	errType := "invalid_request_error"
	if httpStatus >= 500 {
		errType = "internal_error"
	} else if httpStatus == 429 {
		errType = "rate_limit_error"
	}
	
	writeOpenAIError(w, msg, errType, "", httpStatus)
}
