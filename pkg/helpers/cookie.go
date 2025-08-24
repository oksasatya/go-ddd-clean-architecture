package helpers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Manager struct {
	Domain string
	Secure bool
}

func NewCookie(domain string, secure bool) *Manager {
	return &Manager{Domain: domain, Secure: secure}
}

func (m *Manager) SetPair(c *gin.Context, access string, aexp time.Time, refresh string, rexp time.Time) {
	c.SetSameSite(http.SameSiteLaxMode)
	aMax := maxAgeFrom(aexp)
	rMax := maxAgeFrom(rexp)

	c.SetCookie("access_token", access, aMax, "/", m.Domain, m.Secure, true)
	c.SetCookie("refresh_token", refresh, rMax, "/", m.Domain, m.Secure, true)
}

func (m *Manager) Clear(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", "", -1, "/", m.Domain, m.Secure, true)
	c.SetCookie("refresh_token", "", -1, "/", m.Domain, m.Secure, true)
	// Match HttpOnly=true used when setting device_id
	c.SetCookie("device_id", "", -1, "/", m.Domain, m.Secure, true)
}

// SetDeviceID stores a long-lived device identifier cookie used to recognize trusted devices.
func (m *Manager) SetDeviceID(c *gin.Context, deviceID string, exp time.Time) {
	c.SetSameSite(http.SameSiteLaxMode)
	dMax := maxAgeFrom(exp)
	// HttpOnly for better security; sent automatically on requests.
	c.SetCookie("device_id", deviceID, dMax, "/", m.Domain, m.Secure, true)
}

func maxAgeFrom(exp time.Time) int {
	sec := int(time.Until(exp).Seconds())
	if sec < 0 {
		return 0
	}
	return sec
}
