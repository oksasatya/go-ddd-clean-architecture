package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Geo save lookup result
type Geo struct {
	City     string
	Region   string // state/province
	Country  string
	Timezone string
}

type GeoResolver interface {
	Lookup(ctx context.Context, ip string) (Geo, error)
}

func FormatGeo(g Geo) string {
	var parts []string
	if s := strings.TrimSpace(g.City); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(g.Region); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(g.Country); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// IPAPIResolver  implements GeoResolver using ip-api.com
type IPAPIResolver struct {
	Client *http.Client
}

func (r IPAPIResolver) Lookup(ctx context.Context, ip string) (Geo, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return Geo{}, fmt.Errorf("empty ip")
	}
	if r.Client == nil {
		r.Client = &http.Client{Timeout: 2 * time.Second}
	}

	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,regionName,city,timezone", ip)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := r.Client.Do(req)
	if err != nil {
		return Geo{}, err
	}
	defer resp.Body.Close()

	var body struct {
		Status     string `json:"status"`
		Message    string `json:"message"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
		Timezone   string `json:"timezone"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Geo{}, err
	}
	if strings.ToLower(body.Status) != "success" {
		return Geo{}, fmt.Errorf("geo lookup failed: %s", body.Message)
	}
	return Geo{City: body.City, Region: body.RegionName, Country: body.Country, Timezone: body.Timezone}, nil
}
