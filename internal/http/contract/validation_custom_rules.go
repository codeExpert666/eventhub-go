package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"

	"eventhub-go/internal/http/requesterror"
)

const validationLocalDateTimeLayout = "2006-01-02T15:04:05"

type customRuleRequestValues struct {
	request    *http.Request
	pathParams map[string]string
	body       map[string]any
	bodyLoaded bool
	bodyErr    error
}

func validateCustomRules(
	catalog *ValidationCatalog,
	input *openapi3filter.RequestValidationInput,
) (requesterror.Violations, error) {
	operationID := requestValidationOperationID(input)
	if catalog == nil || operationID == "" || input == nil || input.Request == nil {
		return nil, nil
	}

	values := customRuleRequestValues{
		request:    input.Request,
		pathParams: input.PathParams,
	}
	violations := make(requesterror.Violations, 0)
	for _, fieldKey := range catalog.customRuleFields(operationID) {
		value, present, err := values.stringValue(fieldKey)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}

		for _, rule := range catalog.fieldRules[fieldKey] {
			if fieldCustomRulePasses(rule.name, value) {
				continue
			}
			violations = append(violations, requesterror.Violation{
				Location: fieldKey.location,
				Field:    customRuleField(fieldKey.location, fieldKey.field),
				Path:     fieldKey.field,
				Rule:     rule.name,
				Message:  rule.message,
			})
		}
	}

	for _, rule := range catalog.crossFieldRules[operationID] {
		if rule.rule != crossFieldRuleNotAfter || !values.queryNotAfter(rule.left, rule.right) {
			continue
		}
		violations = append(violations, requesterror.Violation{
			Location: requesterror.LocationQuery,
			Field:    rule.left,
			Path:     rule.left,
			Rule:     rule.rule,
			Message:  rule.message,
		})
	}
	return violations, nil
}

func requestValidationOperationID(input *openapi3filter.RequestValidationInput) string {
	if input == nil || input.Route == nil || input.Route.Operation == nil {
		return ""
	}
	return strings.TrimSpace(input.Route.Operation.OperationID)
}

func (catalog *ValidationCatalog) customRuleFields(operationID string) []validationFieldKey {
	if catalog == nil {
		return nil
	}
	operationID = strings.TrimSpace(operationID)
	fields := make([]validationFieldKey, 0)
	for fieldKey := range catalog.fieldRules {
		if fieldKey.operationID == operationID {
			fields = append(fields, fieldKey)
		}
	}
	sort.Slice(fields, func(left, right int) bool {
		leftOrder := customRuleLocationOrder(fields[left].location)
		rightOrder := customRuleLocationOrder(fields[right].location)
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		return fields[left].field < fields[right].field
	})
	return fields
}

func customRuleLocationOrder(location string) int {
	switch location {
	case requesterror.LocationBody:
		return 0
	case requesterror.LocationQuery:
		return 1
	case requesterror.LocationPath:
		return 2
	case requesterror.LocationHeader:
		return 3
	case requesterror.LocationCookie:
		return 4
	default:
		return 5
	}
}

func (values *customRuleRequestValues) stringValue(fieldKey validationFieldKey) (string, bool, error) {
	switch fieldKey.location {
	case requesterror.LocationBody:
		return values.bodyString(fieldKey.field)
	case requesterror.LocationQuery:
		queryValues, present := values.request.URL.Query()[fieldKey.field]
		if !present || len(queryValues) == 0 {
			return "", false, nil
		}
		return queryValues[0], true, nil
	case requesterror.LocationPath:
		value, present := values.pathParams[fieldKey.field]
		return value, present, nil
	case requesterror.LocationHeader:
		headerValues := values.request.Header.Values(fieldKey.field)
		if len(headerValues) == 0 {
			return "", false, nil
		}
		return headerValues[0], true, nil
	case requesterror.LocationCookie:
		cookie, err := values.request.Cookie(fieldKey.field)
		if err == http.ErrNoCookie {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("read custom validation cookie %s: %w", fieldKey.field, err)
		}
		return cookie.Value, true, nil
	default:
		return "", false, nil
	}
}

func (values *customRuleRequestValues) bodyString(path string) (string, bool, error) {
	if err := values.loadBody(); err != nil {
		return "", false, err
	}
	if values.body == nil {
		return "", false, nil
	}

	var current any = values.body
	for _, segment := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return "", false, nil
		}
		current, ok = object[segment]
		if !ok || current == nil {
			return "", false, nil
		}
	}
	value, ok := current.(string)
	return value, ok, nil
}

func (values *customRuleRequestValues) loadBody() error {
	if values.bodyLoaded {
		return values.bodyErr
	}
	values.bodyLoaded = true

	data, err := requestBodyCopy(values.request)
	if err != nil {
		values.bodyErr = fmt.Errorf("read request body for custom validation: %w", err)
		return values.bodyErr
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &values.body); err != nil {
		values.bodyErr = fmt.Errorf("decode request body for custom validation: %w", err)
	}
	return values.bodyErr
}

func requestBodyCopy(request *http.Request) ([]byte, error) {
	if request == nil || request.Body == nil || request.Body == http.NoBody {
		return nil, nil
	}
	if request.GetBody != nil {
		body, err := request.GetBody()
		if err != nil {
			return nil, err
		}
		defer body.Close()
		return io.ReadAll(body)
	}

	data, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	_ = request.Body.Close()
	request.ContentLength = int64(len(data))
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	request.Body, _ = request.GetBody()
	return data, nil
}

func fieldCustomRulePasses(rule, value string) bool {
	switch rule {
	case customRuleNotBlank:
		return strings.TrimSpace(value) != ""
	case customRuleContainsLetterAndDigit:
		return containsASCIILetterAndDigit(value)
	case customRuleLocalDateTime:
		_, err := time.Parse(validationLocalDateTimeLayout, value)
		return err == nil
	default:
		return true
	}
}

func containsASCIILetterAndDigit(value string) bool {
	hasLetter := false
	hasDigit := false
	for _, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') {
			hasLetter = true
		}
		if character >= '0' && character <= '9' {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

func (values *customRuleRequestValues) queryNotAfter(leftField, rightField string) bool {
	query := values.request.URL.Query()
	leftRaw, leftPresent := firstQueryValue(query[leftField])
	rightRaw, rightPresent := firstQueryValue(query[rightField])
	if !leftPresent || !rightPresent {
		return false
	}
	left, leftErr := time.Parse(validationLocalDateTimeLayout, strings.TrimSpace(leftRaw))
	right, rightErr := time.Parse(validationLocalDateTimeLayout, strings.TrimSpace(rightRaw))
	return leftErr == nil && rightErr == nil && left.After(right)
}

func firstQueryValue(values []string) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	return values[0], true
}

func customRuleField(location, path string) string {
	if location != requesterror.LocationBody {
		return path
	}
	field, _, _ := strings.Cut(path, ".")
	return field
}
