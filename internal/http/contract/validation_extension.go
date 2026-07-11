package contract

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	customRuleNotBlank               = "notBlank"
	customRuleContainsLetterAndDigit = "containsLetterAndDigit"
	customRuleLocalDateTime          = "localDateTime"
	crossFieldRuleNotAfter           = "notAfter"
)

type fieldValidationExtension struct {
	notBlank    bool
	messages    map[string]string
	customRules []namedValidationRule
}

type operationValidationExtension struct {
	messages    map[string]string
	crossFields []crossFieldValidationRule
}

type namedValidationRule struct {
	name    string
	message string
}

type crossFieldValidationRule struct {
	name    string
	rule    string
	left    string
	right   string
	message string
}

func parseFieldValidationExtension(schemaRef *openapi3.SchemaRef, label string) (fieldValidationExtension, bool, error) {
	raw, present := rawValidationExtension(schemaRef)
	if !present {
		return fieldValidationExtension{}, false, nil
	}

	value, ok := raw.(map[string]any)
	if !ok {
		return fieldValidationExtension{}, true, fmt.Errorf("%s x-validation must be an object", label)
	}
	if err := rejectUnknownKeys(value, []string{"messages", "notBlank", "rules"}, label+" x-validation"); err != nil {
		return fieldValidationExtension{}, true, err
	}

	extension := fieldValidationExtension{}
	if rawMessages, exists := value["messages"]; exists {
		messages, err := parseValidationMessages(rawMessages, label+" x-validation.messages")
		if err != nil {
			return fieldValidationExtension{}, true, err
		}
		extension.messages = messages
	}

	if rawNotBlank, exists := value["notBlank"]; exists {
		notBlank, ok := rawNotBlank.(bool)
		if !ok || !notBlank {
			return fieldValidationExtension{}, true, fmt.Errorf("%s x-validation.notBlank must be true when declared", label)
		}
		extension.notBlank = true
		if strings.TrimSpace(extension.messages["notBlank"]) == "" {
			return fieldValidationExtension{}, true, fmt.Errorf("%s x-validation.notBlank must declare a non-empty messages.notBlank", label)
		}
	}

	if rawRules, exists := value["rules"]; exists {
		rules, err := parseNamedValidationRules(rawRules, label+" x-validation.rules")
		if err != nil {
			return fieldValidationExtension{}, true, err
		}
		extension.customRules = rules
	}

	return extension, true, nil
}

func parseOperationValidationExtension(operation *openapi3.Operation, label string) (operationValidationExtension, bool, error) {
	if operation == nil || operation.Extensions == nil {
		return operationValidationExtension{}, false, nil
	}
	raw, present := operation.Extensions["x-validation"]
	if !present {
		return operationValidationExtension{}, false, nil
	}

	value, ok := raw.(map[string]any)
	if !ok {
		return operationValidationExtension{}, true, fmt.Errorf("%s x-validation must be an object", label)
	}
	if err := rejectUnknownKeys(value, []string{"crossFields", "messages"}, label+" x-validation"); err != nil {
		return operationValidationExtension{}, true, err
	}

	extension := operationValidationExtension{}
	if rawMessages, exists := value["messages"]; exists {
		messages, err := parseValidationMessages(rawMessages, label+" x-validation.messages")
		if err != nil {
			return operationValidationExtension{}, true, err
		}
		extension.messages = messages
	}
	if rawCrossFields, exists := value["crossFields"]; exists {
		crossFields, err := parseCrossFieldValidationRules(rawCrossFields, label+" x-validation.crossFields")
		if err != nil {
			return operationValidationExtension{}, true, err
		}
		extension.crossFields = crossFields
	}

	return extension, true, nil
}

func rawValidationExtension(schemaRef *openapi3.SchemaRef) (any, bool) {
	if schemaRef == nil {
		return nil, false
	}
	if raw, ok := schemaRef.Extensions["x-validation"]; ok {
		return raw, true
	}
	if schemaRef.Value == nil {
		return nil, false
	}
	raw, ok := schemaRef.Value.Extensions["x-validation"]
	return raw, ok
}

func parseValidationMessages(raw any, label string) (map[string]string, error) {
	values, ok := raw.(map[string]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty string map", label)
	}

	keys := sortedKeys(values)
	messages := make(map[string]string, len(values))
	for _, key := range keys {
		rule := strings.TrimSpace(key)
		message, ok := values[key].(string)
		if rule == "" || !ok || strings.TrimSpace(message) == "" {
			return nil, fmt.Errorf("%s.%s must be a non-empty string", label, key)
		}
		if _, duplicate := messages[rule]; duplicate {
			return nil, fmt.Errorf("%s duplicates rule %q after trimming", label, rule)
		}
		messages[rule] = message
	}
	return messages, nil
}

func parseNamedValidationRules(raw any, label string) ([]namedValidationRule, error) {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty array", label)
	}

	rules := make([]namedValidationRule, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, rawRule := range values {
		value, ok := rawRule.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", label, index)
		}
		itemLabel := fmt.Sprintf("%s[%d]", label, index)
		if err := rejectUnknownKeys(value, []string{"message", "name"}, itemLabel); err != nil {
			return nil, err
		}
		name, err := requiredExtensionString(value, "name", itemLabel)
		if err != nil {
			return nil, err
		}
		message, err := requiredExtensionString(value, "message", itemLabel)
		if err != nil {
			return nil, err
		}
		if name != customRuleContainsLetterAndDigit && name != customRuleLocalDateTime {
			return nil, fmt.Errorf("%s uses unsupported custom rule %q", itemLabel, name)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("%s duplicates custom rule %q", label, name)
		}
		seen[name] = struct{}{}
		rules = append(rules, namedValidationRule{name: name, message: message})
	}
	return rules, nil
}

func parseCrossFieldValidationRules(raw any, label string) ([]crossFieldValidationRule, error) {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty array", label)
	}

	rules := make([]crossFieldValidationRule, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, rawRule := range values {
		value, ok := rawRule.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", label, index)
		}
		itemLabel := fmt.Sprintf("%s[%d]", label, index)
		if err := rejectUnknownKeys(value, []string{"left", "message", "name", "right", "rule"}, itemLabel); err != nil {
			return nil, err
		}
		name, err := requiredExtensionString(value, "name", itemLabel)
		if err != nil {
			return nil, err
		}
		rule, err := requiredExtensionString(value, "rule", itemLabel)
		if err != nil {
			return nil, err
		}
		left, err := requiredExtensionString(value, "left", itemLabel)
		if err != nil {
			return nil, err
		}
		right, err := requiredExtensionString(value, "right", itemLabel)
		if err != nil {
			return nil, err
		}
		message, err := requiredExtensionString(value, "message", itemLabel)
		if err != nil {
			return nil, err
		}
		if rule != crossFieldRuleNotAfter {
			return nil, fmt.Errorf("%s uses unsupported custom rule %q", itemLabel, rule)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("%s duplicates cross-field rule %q", label, name)
		}
		seen[name] = struct{}{}
		rules = append(rules, crossFieldValidationRule{
			name:    name,
			rule:    rule,
			left:    left,
			right:   right,
			message: message,
		})
	}
	return rules, nil
}

func requiredExtensionString(value map[string]any, key, label string) (string, error) {
	raw, ok := value[key].(string)
	result := strings.TrimSpace(raw)
	if !ok || result == "" {
		return "", fmt.Errorf("%s.%s must be a non-empty string", label, key)
	}
	return result, nil
}

func rejectUnknownKeys(value map[string]any, allowed []string, label string) error {
	allowedKeys := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedKeys[key] = struct{}{}
	}
	for _, key := range sortedKeys(value) {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("%s.%s is not allowed", label, key)
		}
	}
	return nil
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
