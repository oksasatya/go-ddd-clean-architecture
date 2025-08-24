package templates

import (
	"context"
	"strings"
	"time"

	"github.com/oksasatya/go-ddd-clean-architecture/config"
)

// Option pattern
type Option func(*EmailData)

func WithIP(ip string) Option        { return func(d *EmailData) { d.IP = ip } }
func WithUserAgent(ua string) Option { return func(d *EmailData) { d.UserAgent = ua } }
func WithTime(t time.Time) Option {
	return func(d *EmailData) {
		utc := t.UTC()
		d.TimeAt = utc
		d.Time = utc.Format("02 January 2006, 15:04")
	}
}
func WithVerifyURL(url string) Option { return func(d *EmailData) { d.VerifyURL = url } }
func WithResetURL(url string) Option  { return func(d *EmailData) { d.ResetURL = url } }
func WithChanges(ch map[string]string) Option {
	return func(d *EmailData) { d.Changes = ch }
}

func setLocation(d *EmailData, loc string) {
	if s := strings.TrimSpace(loc); s != "" {
		d.Location = s
	}
}

func WithLocation(loc string) Option {
	return func(d *EmailData) { setLocation(d, loc) }
}

func WithGeo(g Geo) Option {
	return func(d *EmailData) { setLocation(d, FormatGeo(g)) }
}

func WithGeoFromIP(ctx context.Context, r GeoResolver, ip string) Option {
	return func(d *EmailData) {
		if r == nil || strings.TrimSpace(ip) == "" {
			return
		}
		if g, err := r.Lookup(ctx, ip); err == nil {
			setLocation(d, FormatGeo(g))
		}
	}
}

func WithExpiresAt(t time.Time) Option {
	return func(d *EmailData) {
		utc := t.UTC()
		d.ExpiresAt = utc
		d.ExpiresAtText = utc.Format("02 January 2006, 15:04")
	}
}

func WithExpiresIn(dur time.Duration) Option {
	return func(d *EmailData) {
		utc := time.Now().Add(dur).UTC()
		d.ExpiresAt = utc
		d.ExpiresAtText = utc.Format("02 January 2006, 15:04")
	}
}

// NewBaseEmailData mengisi field umum dari config, lalu apply Option
func NewBaseEmailData(cfg *config.Config, typ string, name, email, recipient string, opts ...Option) EmailData {
	d := EmailData{
		Name:           name,
		Email:          email,
		RecipientEmail: recipient,
		Type:           typ,

		CompanyName:    cfg.CompanyName,
		CompanyAddress: cfg.CompanyAddress,
		AppName:        cfg.AppName,

		LogoURL:        cfg.LogoURL,
		SupportURL:     cfg.SupportURL,
		PrivacyURL:     cfg.PrivacyURL,
		UnsubscribeURL: cfg.UnsubscribeURL,

		ResetURL:  cfg.ResetPasswordURL,
		VerifyURL: cfg.VerifyEmailURL,
	}
	for _, opt := range opts {
		opt(&d)
	}
	return d
}

func NewLoginNotificationData(cfg *config.Config, name, email, recipientEmail string, opts ...Option) map[string]any {
	d := NewBaseEmailData(cfg, LoginNotification, name, email, recipientEmail, opts...)
	return ToMap(d)
}

func NewVerifyEmailData(cfg *config.Config, name, email, verifyURL string, opts ...Option) map[string]any {
	opts = append([]Option{WithVerifyURL(verifyURL)}, opts...)
	d := NewBaseEmailData(cfg, VerifyEmail, name, email, email, opts...)
	return ToMap(d)
}

func NewForgotPasswordData(cfg *config.Config, name, email, recipient string, opts ...Option) map[string]any {
	d := NewBaseEmailData(cfg, ForgotPassword, name, email, recipient, opts...)
	return ToMap(d)
}

func NewProfileUpdatedData(cfg *config.Config, name, email string, changes map[string]string, opts ...Option) map[string]any {
	opts = append([]Option{WithChanges(changes)}, opts...)
	d := NewBaseEmailData(cfg, ProfileUpdated, name, email, email, opts...)
	return ToMap(d)
}

func NewLoginOTPData(cfg *config.Config, name, email, code string, opts ...Option) map[string]any {
	// put code and expires into data
	base := NewBaseEmailData(cfg, LoginOTP, name, email, email, opts...)
	base.Code = code
	return ToMap(base)
}
