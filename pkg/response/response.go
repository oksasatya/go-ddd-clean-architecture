package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type APIResponse[T any] struct {
	Status    int         `json:"status"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id"`
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      T           `json:"data,omitempty"`
	Meta      interface{} `json:"meta,omitempty"`
	Error     interface{} `json:"error,omitempty"`
}

func Success[T any](ctx *gin.Context, status int, data T, message string, meta interface{}) APIResponse[T] {
	if status == 0 {
		status = http.StatusOK
	}
	return APIResponse[T]{
		Status:    status,
		Timestamp: time.Now(),
		RequestID: ctx.GetString("request_id"),
		Success:   true,
		Message:   message,
		Data:      data,
		Meta:      meta,
	}
}

func Error[T any](ctx *gin.Context, status int, message string, err interface{}) APIResponse[T] {
	if status == 0 {
		status = http.StatusBadRequest
	}
	return APIResponse[T]{
		Status:    status,
		Timestamp: time.Now(),
		RequestID: ctx.GetString("request_id"),
		Success:   false,
		Message:   message,
		Error:     err,
	}
}
