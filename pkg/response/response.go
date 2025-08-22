package response

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Meta struct {
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
	Status    int       `json:"status"`
	IP        string    `json:"ip"`
	OS        string    `json:"os"`
}

type ErrorBody struct {
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type Envelope[T any] struct {
	Meta  Meta       `json:"meta"`
	Data  T          `json:"data,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

func makeMeta(ctx *gin.Context, status int) Meta {
	if status == 0 {
		status = http.StatusOK
	}
	ua := ctx.GetHeader("User-Agent")

	ip := ctx.GetString("real_ip")
	if ip == "" || net.ParseIP(ip) == nil {
		ip = ctx.ClientIP()
	}

	return Meta{
		RequestID: ctx.GetString("request_id"),
		Timestamp: time.Now().UTC().Round(time.Millisecond),
		Status:    status,
		IP:        ip,
		OS:        parseOSFromUA(ua),
	}
}

// Success responds with the standard envelope. The `message` and `meta` parameters are ignored to preserve call sites.
func Success[T any](ctx *gin.Context, status int, data T, _ string, _ interface{}) Envelope[T] {
	m := makeMeta(ctx, status)
	env := Envelope[T]{Meta: m, Data: data}
	ctx.JSON(m.Status, env)
	return env
}

// Error responds with the standard envelope carrying an error body. The `err` parameter is used as details.
func Error[T any](ctx *gin.Context, status int, message string, err interface{}) Envelope[T] {
	if status == 0 {
		status = http.StatusBadRequest
	}
	m := makeMeta(ctx, status)
	body := &ErrorBody{Message: message}
	if err != nil {
		body.Details = err
	}
	env := Envelope[T]{Meta: m, Error: body}
	ctx.JSON(m.Status, env)
	return env
}

// parseOSFromUA extracts a friendly OS string from User-Agent; best-effort.
func parseOSFromUA(ua string) string {
	if ua == "" {
		return "Unknown"
	}
	// Attempt to extract text within the first parentheses
	start := strings.Index(ua, "(")
	end := strings.Index(ua, ")")
	inner := ""
	if start != -1 && end != -1 && end > start+1 {
		inner = ua[start+1 : end]
	} else {
		inner = ua
	}

	in := inner
	lower := strings.ToLower(in)

	// Windows mapping
	if strings.Contains(lower, "windows nt 11.0") {
		return "Windows 11"
	}
	if strings.Contains(lower, "windows nt 10.0") {
		return "Windows 10"
	}
	if strings.Contains(lower, "windows nt 6.3") {
		return "Windows 8.1"
	}
	if strings.Contains(lower, "windows nt 6.1") {
		return "Windows 7"
	}

	// Mac OS X mapping
	if idx := strings.Index(in, "Mac OS X "); idx != -1 {
		v := in[idx+len("Mac OS X "):]
		// Trim after first semicolon or closing
		if semi := strings.IndexAny(v, ";)"); semi != -1 {
			v = v[:semi]
		}
		v = strings.TrimSpace(v)
		v = strings.ReplaceAll(v, "_", ".")
		if v != "" {
			return "Mac OS X " + v
		}
		return "Mac OS X"
	}

	// iOS
	if idx := strings.Index(in, "CPU iPhone OS "); idx != -1 {
		v := in[idx+len("CPU iPhone OS "):]
		if semi := strings.IndexAny(v, ";)"); semi != -1 {
			v = v[:semi]
		}
		v = strings.TrimSpace(v)
		v = strings.ReplaceAll(v, "_", ".")
		if v != "" {
			return "iOS " + v
		}
		return "iOS"
	}

	// Android
	if idx := strings.Index(in, "Android "); idx != -1 {
		v := in[idx+len("Android "):]
		if semi := strings.IndexAny(v, ";)"); semi != -1 {
			v = v[:semi]
		}
		v = strings.TrimSpace(v)
		if v != "" {
			return "Android " + v
		}
		return "Android"
	}

	// Fallback: return the inner parenthetical section or generic Unknown
	if inner != "" {
		return inner
	}
	return "Unknown"
}
