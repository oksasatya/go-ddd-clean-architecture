package helpers

import (
	"context"
	"fmt"
	"strings"
	"time"

	mailtpl "github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer/templates"
)

func LocalizeTimesIfPossible(ctx context.Context, resolver mailtpl.GeoResolver, data map[string]any) {
	ipVal, ok := data["IP"]
	if !ok || fmt.Sprintf("%v", ipVal) == "" {
		return
	}
	ip := fmt.Sprintf("%v", ipVal)
	g, err := resolver.Lookup(ctx, ip)
	if err != nil || strings.TrimSpace(g.Timezone) == "" {
		return
	}
	loc, err := time.LoadLocation(g.Timezone)
	if err != nil {
		return
	}
	// ExpiresAt
	if v, ok := data["ExpiresAt"]; ok {
		if t, ok2 := parseTimeAny(v); ok2 {
			local := t.In(loc)
			data["ExpiresAtText"] = local.Format("02 January 2006, 15:04 MST")
		}
	}
	// TimeAt -> Time
	if v, ok := data["TimeAt"]; ok {
		if t, ok2 := parseTimeAny(v); ok2 {
			local := t.In(loc)
			data["Time"] = local.Format("02 January 2006, 15:04 MST")
		}
	}
}

func parseTimeAny(v any) (time.Time, bool) {
	s := fmt.Sprintf("%v", v)
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
