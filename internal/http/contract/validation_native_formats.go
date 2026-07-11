package contract

import (
	"errors"
	"net/mail"
	"strings"
	"sync"
	"unicode/utf16"

	"github.com/getkin/kin-openapi/openapi3"
)

var registerNativeFormatsOnce sync.Once

const maxEmailLocalPartLength = 64

func registerNativeFormats() {
	registerNativeFormatsOnce.Do(func() {
		openapi3.DefineStringFormatValidator(
			"email",
			openapi3.NewCallbackValidator(validateEmailFormat),
		)
	})
}

func validateEmailFormat(value string) error {
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return errors.New("invalid email format")
	}
	at := strings.LastIndexByte(value, '@')
	if at <= 0 || at == len(value)-1 || utf16CodeUnitCount(value[:at]) > maxEmailLocalPartLength {
		return errors.New("invalid email format")
	}
	return nil
}

func utf16CodeUnitCount(value string) int {
	count := 0
	for _, character := range value {
		count += utf16.RuneLen(character)
	}
	return count
}
