package contract

import (
	"errors"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
)

func appErrorFromValidationError(err error, catalog *ValidationCatalog) *apperror.AppError {
	if appErr := appErrorFromSecurityError(err); appErr != nil {
		return appErr
	}

	var requestErr *openapi3filter.RequestError
	if !errors.As(err, &requestErr) {
		return requesterror.InvalidParameters(requesterror.Violations{{
			Location: requesterror.LocationQuery,
			Field:    "request",
			Path:     "request",
			Rule:     "contract",
			Message:  "请求不符合 OpenAPI 契约",
		}})
	}

	if parameter := requestErr.Parameter; parameter != nil {
		violations := requesterror.Violations{parameterViolation(catalog, requestErr)}
		switch parameter.In {
		case openapi3.ParameterInHeader:
			return requesterror.InvalidHeaders(violations)
		case openapi3.ParameterInCookie:
			return requesterror.InvalidCookies(violations)
		default:
			return requesterror.InvalidParameters(violations)
		}
	}

	if requestErr.RequestBody != nil {
		if unsupportedContentType(requestErr) {
			return requesterror.UnsupportedContentType(contentType(requestErr))
		}
		if errors.Is(requestErr.Err, openapi3filter.ErrInvalidRequired) {
			return requesterror.MissingBody()
		}
		if malformedBody(requestErr) {
			return requesterror.MalformedBody()
		}
		return requesterror.InvalidBody(bodyFieldErrors(catalog, requestErr))
	}

	return requesterror.InvalidParameters(requesterror.Violations{{
		Location: requesterror.LocationQuery,
		Field:    "request",
		Path:     "request",
		Rule:     "contract",
		Message:  "请求不符合 OpenAPI 契约",
	}})
}

func appErrorFromCustomRuleViolations(violations requesterror.Violations) *apperror.AppError {
	if len(violations) == 0 {
		return nil
	}
	location := violations[0].Location
	for _, violation := range violations[1:] {
		if violation.Location != location {
			return requesterror.InvalidParameters(violations)
		}
	}

	switch location {
	case requesterror.LocationBody:
		return requesterror.InvalidBody(violations)
	case requesterror.LocationHeader:
		return requesterror.InvalidHeaders(violations)
	case requesterror.LocationCookie:
		return requesterror.InvalidCookies(violations)
	default:
		return requesterror.InvalidParameters(violations)
	}
}

func unsupportedContentType(requestErr *openapi3filter.RequestError) bool {
	if strings.Contains(requestErr.Reason, "Content-Type") {
		return true
	}
	var parseErr *openapi3filter.ParseError
	return errors.As(requestErr.Err, &parseErr) &&
		(parseErr.Kind == openapi3filter.KindUnsupportedFormat || strings.Contains(parseErr.Reason, "unsupported content type"))
}

func contentType(requestErr *openapi3filter.RequestError) string {
	if requestErr.Input == nil || requestErr.Input.Request == nil {
		return ""
	}
	return requestErr.Input.Request.Header.Get("Content-Type")
}

func malformedBody(requestErr *openapi3filter.RequestError) bool {
	if strings.Contains(requestErr.Reason, "failed to decode request body") {
		return true
	}
	var parseErr *openapi3filter.ParseError
	if !errors.As(requestErr.Err, &parseErr) {
		return false
	}
	return parseErr.Kind != openapi3filter.KindUnsupportedFormat
}
