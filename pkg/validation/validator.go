package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Init configures the global validator used by Gin's binding.
// - Uses JSON tag names in errors.
// - Registers alias tags for common validations.
func Init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
		// Aliases for common semantics
		v.RegisterAlias("pwd", "min=8") // password minimum length
		v.RegisterAlias("strongpwd", "min=8,containsany=!@#$%^&*(),containsany=0123456789,containsany=ABCDEFGHIJKLMNOPQRSTUVWXYZ,containsany=abcdefghijklmnopqrstuvwxyz")
		v.RegisterAlias("uuid4", "uuid")       // keep uuid as base; many use uuid4 synonym
		v.RegisterAlias("nonzero", "required") // convenience
		v.RegisterAlias("phone", "e164")       // phone number alias
	}
}

// ToDetails converts validation/binding errors into a map[field]message suitable for API error.details.
// It covers all validator.v10 tags to provide stable, human-friendly messages.
func ToDetails(err error) map[string]string {
	if err == nil {
		return nil
	}

	// Invalid JSON payloads
	var se *json.SyntaxError
	var ute *json.UnmarshalTypeError
	if errors.As(err, &se) || errors.As(err, &ute) {
		return map[string]string{"payload": "invalid json"}
	}

	// Validation errors from validator.v10
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		out := make(map[string]string, len(verrs))
		for _, fe := range verrs {
			field := fe.Field()
			out[field] = formatFieldError(fe)
		}
		return out
	}

	// Fallback
	return map[string]string{"payload": "invalid payload"}
}

func formatFieldError(fe validator.FieldError) string {
	tag := fe.Tag()
	param := fe.Param()
	kind := fe.Kind()

	switch tag {
	// ===== PRESENCE/REQUIRED VALIDATIONS =====
	case "required":
		return "is required"
	case "required_with":
		return "is required when " + param + " is present"
	case "required_with_all":
		return "is required when all of " + param + " are present"
	case "required_without":
		return "is required when " + param + " is not present"
	case "required_without_all":
		return "is required when none of " + param + " are present"
	case "required_if":
		return "is required if " + param
	case "required_unless":
		return "is required unless " + param
	case "omitempty":
		return "can be omitted"

	// ===== EXCLUSION VALIDATIONS =====
	case "excluded_with":
		return "must be excluded when " + param + " is present"
	case "excluded_without":
		return "must be excluded when " + param + " is not present"
	case "excluded_with_all":
		return "must be excluded when all of " + param + " are present"
	case "excluded_without_all":
		return "must be excluded when none of " + param + " are present"
	case "excluded_if":
		return "must be excluded if " + param
	case "excluded_unless":
		return "must be excluded unless " + param

	// ===== STRING FORMAT VALIDATIONS =====
	case "email":
		return "must be a valid email"
	case "url":
		return "must be a valid URL"
	case "uri":
		return "must be a valid URI"
	case "urn_rfc2141":
		return "must be a valid URN format"
	case "file":
		return "must be a valid file path"
	case "base64":
		return "must be properly base64 encoded"
	case "base64url":
		return "must be properly base64url encoded"
	case "base64rawurl":
		return "must be properly base64 raw URL encoded"
	case "datauri":
		return "must be a valid data URI"
	case "isbn":
		return "must be a valid ISBN number"
	case "isbn10":
		return "must be a valid ISBN-10 number"
	case "isbn13":
		return "must be a valid ISBN-13 number"

	// ===== UUID/ULID VALIDATIONS =====
	case "uuid":
		return "must be a valid UUID"
	case "uuid3":
		return "must be a valid UUID version 3"
	case "uuid4":
		return "must be a valid UUID version 4"
	case "uuid5":
		return "must be a valid UUID version 5"
	case "uuid_rfc4122":
		return "must be a valid UUID format"
	case "ulid":
		return "must be a valid ULID"

	// ===== PHONE NUMBER VALIDATIONS =====
	case "e164":
		return "must be a valid phone number"

	// ===== CHARACTER SET VALIDATIONS =====
	case "alpha":
		return "must contain alphabetic characters only"
	case "alphanum":
		return "must contain alphanumeric characters only"
	case "alphanumunicode":
		return "must contain alphanumeric (unicode) characters only"
	case "alphaunicode":
		return "must contain alphabetic (unicode) characters only"
	case "ascii":
		return "must contain ASCII characters only"
	case "printascii":
		return "must contain printable ASCII characters only"
	case "multibyte":
		return "must contain multibyte characters"
	case "lowercase":
		return "must be in lowercase"
	case "uppercase":
		return "must be in uppercase"

	// ===== STRING CONTENT VALIDATIONS =====
	case "contains":
		return "must contain '" + param + "'"
	case "containsany":
		return "must contain at least one of '" + param + "'"
	case "containsrune":
		return "must contain the rune '" + param + "'"
	case "excludes":
		return "must not contain '" + param + "'"
	case "excludesall":
		return "must not contain any of '" + param + "'"
	case "excludesrune":
		return "must not contain the rune '" + param + "'"
	case "startswith":
		return "must start with '" + param + "'"
	case "endswith":
		return "must end with '" + param + "'"

	// ===== SIZE/LENGTH VALIDATIONS =====
	case "len":
		if param != "" {
			return fmt.Sprintf("must be exactly %s characters long", param)
		}
		return "invalid length"
	case "min":
		if param != "" {
			if isNumberKind(kind) {
				return "must be at least " + param
			}
			return "must be at least " + param + " characters long"
		}
		return "too small"
	case "max":
		if param != "" {
			if isNumberKind(kind) {
				return "must be at most " + param
			}
			return "must be at most " + param + " characters long"
		}
		return "too large"

	// ===== NUMERIC COMPARISON VALIDATIONS =====
	case "eq":
		if param != "" {
			return "must be equal to " + param
		}
		return "must be equal"
	case "ne":
		if param != "" {
			return "must not be equal to " + param
		}
		return "must not be equal"
	case "lt":
		if param != "" {
			return "must be less than " + param
		}
		return "must be less than"
	case "lte":
		if param != "" {
			return "must be less than or equal to " + param
		}
		return "must be less than or equal"
	case "gt":
		if param != "" {
			return "must be greater than " + param
		}
		return "must be greater than"
	case "gte":
		if param != "" {
			return "must be greater than or equal to " + param
		}
		return "must be greater than or equal"

	// ===== FIELD COMPARISON VALIDATIONS =====
	case "eqfield":
		return "must be equal to " + param + " field"
	case "nefield":
		return "must not be equal to " + param + " field"
	case "ltfield":
		return "must be less than " + param + " field"
	case "ltefield":
		return "must be less than or equal to " + param + " field"
	case "gtfield":
		return "must be greater than " + param + " field"
	case "gtefield":
		return "must be greater than or equal to " + param + " field"

	// ===== CROSS STRUCT FIELD VALIDATIONS =====
	case "eqcsfield":
		return "must be equal to " + param + " field"
	case "necsfield":
		return "must not be equal to " + param + " field"
	case "ltcsfield":
		return "must be less than " + param + " field"
	case "ltecsfield":
		return "must be less than or equal to " + param + " field"
	case "gtcsfield":
		return "must be greater than " + param + " field"
	case "gtecsfield":
		return "must be greater than or equal to " + param + " field"

	// ===== TIME FIELD VALIDATIONS =====
	case "gtfield_time":
		return "must be after " + param + " field"
	case "gtefieldtime":
		return "must be at or after " + param + " field"
	case "ltfieldtime":
		return "must be before " + param + " field"
	case "ltefieldtime":
		return "must be at or before " + param + " field"

	// ===== INCLUSION/EXCLUSION VALIDATIONS =====
	case "oneof":
		return "must be one of: " + strings.Join(splitParams(param), ", ")

	// ===== NUMERIC TYPE VALIDATIONS =====
	case "number":
		return "must be a valid number"
	case "numeric":
		return "must be numeric"

	// ===== IP ADDRESS VALIDATIONS =====
	case "ip":
		return "must be a valid IP address"
	case "ipv4":
		return "must be a valid IPv4 address"
	case "ipv6":
		return "must be a valid IPv6 address"
	case "tcp_addr":
		return "must be a valid TCP address"
	case "tcp4_addr":
		return "must be a valid TCPv4 address"
	case "tcp6_addr":
		return "must be a valid TCPv6 address"
	case "udp_addr":
		return "must be a valid UDP address"
	case "udp4_addr":
		return "must be a valid UDPv4 address"
	case "udp6_addr":
		return "must be a valid UDPv6 address"
	case "ip_addr":
		return "must be a valid IP address"
	case "ip4_addr":
		return "must be a valid IPv4 address"
	case "ip6_addr":
		return "must be a valid IPv6 address"
	case "unix_addr":
		return "must be a valid Unix domain socket address"

	// ===== CIDR VALIDATIONS =====
	case "cidr":
		return "must be a valid CIDR notation"
	case "cidrv4":
		return "must be a valid IPv4 CIDR notation"
	case "cidrv6":
		return "must be a valid IPv6 CIDR notation"

	// ===== MAC ADDRESS VALIDATIONS =====
	case "mac":
		return "must be a valid MAC address"

	// ===== HOSTNAME/DOMAIN VALIDATIONS =====
	case "hostname":
		return "must be a valid hostname"
	case "hostname_port":
		return "must be a valid hostname with port"
	case "hostname_rfc1123":
		return "must be a valid hostname format"
	case "fqdn":
		return "must be a valid domain name"

	// ===== DATE/TIME VALIDATIONS =====
	case "datetime":
		if param != "" {
			return "must match datetime format: " + param
		}
		return "must be a valid datetime"
	case "timezone":
		return "must be a valid timezone"

	// ===== HASH VALIDATIONS =====
	case "md4":
		return "must be a valid MD4 hash"
	case "md5":
		return "must be a valid MD5 hash"
	case "sha256":
		return "must be a valid SHA256 hash"
	case "sha384":
		return "must be a valid SHA384 hash"
	case "sha512":
		return "must be a valid SHA512 hash"

	// ===== COLOR VALIDATIONS =====
	case "hexcolor":
		return "must be a valid hexadecimal color"
	case "hexadecimal":
		return "must be hexadecimal"
	case "rgb":
		return "must be a valid RGB color"
	case "rgba":
		return "must be a valid RGBA color"
	case "hsl":
		return "must be a valid HSL color"
	case "hsla":
		return "must be a valid HSLA color"

	// ===== FINANCIAL VALIDATIONS =====
	case "credit_card":
		return "must be a valid credit card number"

	// ===== COLLECTION/SLICE VALIDATIONS =====
	case "unique":
		return "must contain unique items"
	case "dive":
		return "array validation failed"

	// ===== BOOLEAN VALIDATIONS =====
	case "boolean":
		return "must be a boolean value"

	// ===== JSON VALIDATIONS =====
	case "json":
		return "must be valid JSON"

	// ===== JWT VALIDATIONS =====
	case "jwt":
		return "must be a valid JWT token"

	// ===== SEMVER VALIDATIONS =====
	case "semver":
		return "must be a valid semantic version"

	// ===== BIC VALIDATIONS =====
	case "bic":
		return "must be a valid bank code"

	// ===== LATITUDE/LONGITUDE VALIDATIONS =====
	case "latitude":
		return "must be a valid latitude"
	case "longitude":
		return "must be a valid longitude"

	// ===== SSNS AND IDS =====
	case "ssn":
		return "must be a valid social security number"

	// ===== CUSTOM ALIASES =====
	case "pwd":
		return "min length 8"
	case "strongpwd":
		return "must be at least 8 characters with uppercase, lowercase, number and special character"
	case "phone":
		return "must be a valid phone number"

	// ===== DEFAULT FALLBACK =====
	default:
		// For unknown tags, try to provide a meaningful message
		if param != "" {
			return fmt.Sprintf("validation failed for '%s' with parameter '%s'", tag, param)
		}
		return fmt.Sprintf("validation failed for '%s'", tag)
	}
}

// Helper functions
func isNumberKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func splitParams(p string) []string {
	if p == "" {
		return nil
	}
	// Handle space-separated values
	parts := strings.Fields(p)
	if len(parts) > 1 {
		return parts
	}
	// Handle comma-separated values
	if strings.Contains(p, ",") {
		return strings.Split(p, ",")
	}
	// Handle pipe-separated values
	if strings.Contains(p, "|") {
		return strings.Split(p, "|")
	}
	// Single value
	return []string{p}
}

// ValidationsError represents a structured validation error
type ValidationsError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}
