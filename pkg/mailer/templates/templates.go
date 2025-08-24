package templates

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	htmpl "html/template"
	"reflect"
	"strings"
	texttpl "text/template"
	"time"
)

//go:embed *.tmpl
var FS embed.FS

type EmailType string

// EmailData defines standard fields for email templates.
type EmailData struct {
	// Basic info
	Name           string `json:"Name"`
	Email          string `json:"Email"`
	RecipientEmail string `json:"RecipientEmail"`
	Type           string `json:"Type"`

	// Company info
	CompanyName    string `json:"CompanyName"`
	CompanyAddress string `json:"CompanyAddress"`
	AppName        string `json:"AppName"`

	// URLs
	LogoURL        string `json:"LogoURL"`
	SupportURL     string `json:"SupportURL"`
	PrivacyURL     string `json:"PrivacyURL"`
	UnsubscribeURL string `json:"UnsubscribeURL"`

	// Action URLs
	ResetURL  string `json:"ResetURL"`
	VerifyURL string `json:"VerifyURL"`

	// Additional data
	ExpiresAt     time.Time         `json:"ExpiresAt"`
	ExpiresAtText string            `json:"ExpiresAtText"`
	IP            string            `json:"IP"`
	Time          string            `json:"Time"`
	TimeAt        time.Time         `json:"TimeAt"`
	UserAgent     string            `json:"UserAgent"`
	Location      string            `json:"Location"`
	Changes       map[string]string `json:"Changes"`
	Code          string            `json:"Code"` // for OTP codes
}

// ToMap converts EmailData to a map[string]any for EmailJob.Data
func ToMap(d EmailData) map[string]any {
	b, _ := json.Marshal(d)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// defaultFn supports pipe usage: {{ .Value | default "Fallback" }}
func defaultFn(fallback any, value any) any {
	switch x := value.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return fallback
		}
		return x
	case nil:
		return fallback
	default:
		rv := reflect.ValueOf(value)
		if !rv.IsValid() {
			return fallback
		}
		zero := reflect.Zero(rv.Type()).Interface()
		if reflect.DeepEqual(value, zero) {
			return fallback
		}
		return value
	}
}

// ---- FuncMaps (dibangun dari satu sumber) ----

func baseFuncs() map[string]any {
	return map[string]any{
		"now":        func() time.Time { return time.Now().UTC() },
		"formatTime": func(t time.Time, layout string) string { return t.Format(layout) },
		"upper":      strings.ToUpper,
		"default":    defaultFn,
	}
}

var (
	htmlFuncMap = htmpl.FuncMap(baseFuncs())
	textFuncMap = texttpl.FuncMap(baseFuncs())
)

// ---- Template names ----

const (
	LoginNotification = "login_notification"
	VerifyEmail       = "verify_email"
	ForgotPassword    = "forgot_password"
	ProfileUpdated    = "profile_updated"
	LoginOTP          = "login_otp"
)

// renderFile loads and renders a single template file from the embedded FS.
// isHTML indicates whether to use html/template (true) or text/template (false).
func renderFile(filename string, isHTML bool, data any) (string, error) {
	var (
		buf bytes.Buffer
		err error
	)

	if isHTML {
		tpl, e := htmpl.New(filename).Funcs(htmlFuncMap).ParseFS(FS, filename)
		if e != nil {
			return "", fmt.Errorf("parse html %q: %w", filename, e)
		}
		err = tpl.Execute(&buf, data)
	} else {
		tpl, e := texttpl.New(filename).Funcs(textFuncMap).ParseFS(FS, filename)
		if e != nil {
			return "", fmt.Errorf("parse text %q: %w", filename, e)
		}
		err = tpl.Execute(&buf, data)
	}
	if err != nil {
		return "", fmt.Errorf("exec %q: %w", filename, err)
	}
	return buf.String(), nil
}

// Render loads and renders subject, text, and html templates for the given base name.
// Expects: <name>.subject.tmpl, <name>.text.tmpl, <name>.html.tmpl
func Render(name string, data any) (subject string, text string, html string, err error) {
	subject, err = renderFile(name+".subject.tmpl", false, data)
	if err != nil {
		return "", "", "", err
	}
	text, err = renderFile(name+".text.tmpl", false, data)
	if err != nil {
		return "", "", "", err
	}
	html, err = renderFile(name+".html.tmpl", true, data)
	if err != nil {
		return "", "", "", err
	}
	return subject, text, html, nil
}

// RenderHTML renders just an HTML template: <name>.html.tmpl
func RenderHTML(name string, data any) (string, error) {
	return renderFile(name+".html.tmpl", true, data)
}
