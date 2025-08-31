package validation

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	localeen "github.com/go-playground/locales/en"
	localeid "github.com/go-playground/locales/id"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	entrans "github.com/go-playground/validator/v10/translations/en"
	idtrans "github.com/go-playground/validator/v10/translations/id"
)

var (
	trans ut.Translator
)

// Init configures the global validator used by Gin's binding.
// - Uses JSON tag names in errors.
// - Registers alias tags for common validations.
// - Installs universal-translator default translations based on locale ("en", "id").
func Init(locale string) {
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

		// Setup universal translator
		en := localeen.New()
		id := localeid.New()
		uni := ut.New(en, en, id)

		var ok bool
		if locale == "id" {
			trans, ok = uni.GetTranslator("id")
			if ok {
				_ = idtrans.RegisterDefaultTranslations(v, trans)
			}
		}
		if trans == nil { // fallback to en
			trans, ok = uni.GetTranslator("en")
			if ok {
				_ = entrans.RegisterDefaultTranslations(v, trans)
			}
		}
	}
}

// ToDetails converts validation/binding errors into a map[field]message suitable for API error.details.
// Uses registered translations when available.
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
		if trans != nil {
			for field, msg := range verrs.Translate(trans) {
				out[field] = msg
			}
			return out
		}
		// Fallback without translator
		for _, fe := range verrs {
			out[fe.Field()] = fe.Error()
		}
		return out
	}

	// Fallback
	return map[string]string{"payload": "invalid payload"}
}

// ValidationsError represents a structured validation error
type ValidationsError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}
