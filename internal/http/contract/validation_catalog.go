package contract

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"eventhub-go/internal/http/requesterror"
)

// ValidationCatalog 是启动期从 OpenAPI x-validation 编译出的只读字段校验目录。
type ValidationCatalog struct {
	messages        map[validationMessageKey]string
	fieldRules      map[validationFieldKey][]namedValidationRule
	crossFieldRules map[string][]crossFieldValidationRule
}

type validationMessageKey struct {
	operationID string
	location    string
	field       string
	rule        string
}

type validationFieldKey struct {
	operationID string
	location    string
	field       string
}

func compileValidationCatalog(document *openapi3.T) (*ValidationCatalog, error) {
	if document == nil {
		return nil, fmt.Errorf("openapi document is nil")
	}

	catalog := &ValidationCatalog{
		messages:        make(map[validationMessageKey]string),
		fieldRules:      make(map[validationFieldKey][]namedValidationRule),
		crossFieldRules: make(map[string][]crossFieldValidationRule),
	}
	if document.Paths == nil {
		return catalog, nil
	}

	paths := make([]string, 0, document.Paths.Len())
	for path := range document.Paths.Map() {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem := document.Paths.Value(path)
		if pathItem == nil {
			continue
		}
		operations := pathItem.Operations()
		methods := make([]string, 0, len(operations))
		for method := range operations {
			methods = append(methods, method)
		}
		sort.Strings(methods)
		for _, method := range methods {
			operation := operations[method]
			if err := catalog.compileOperation(method, path, pathItem, operation); err != nil {
				return nil, err
			}
		}
	}
	return catalog, nil
}

func (catalog *ValidationCatalog) compileOperation(method, path string, pathItem *openapi3.PathItem, operation *openapi3.Operation) error {
	if operation == nil {
		return nil
	}
	label := strings.ToUpper(method) + " " + path
	operationID := strings.TrimSpace(operation.OperationID)

	operationExtension, present, err := parseOperationValidationExtension(operation, label)
	if err != nil {
		return fmt.Errorf("compile validation catalog for %s: %w", label, err)
	}
	if present {
		if operationID == "" {
			return fmt.Errorf("compile validation catalog for %s: operationId is required when x-validation is declared", label)
		}
		for _, rule := range sortedStringKeys(operationExtension.messages) {
			if err := catalog.addMessage(validationMessageKey{
				operationID: operationID,
				rule:        rule,
			}, operationExtension.messages[rule]); err != nil {
				return fmt.Errorf("compile validation catalog for %s: %w", label, err)
			}
		}
		if len(operationExtension.crossFields) > 0 {
			catalog.crossFieldRules[operationID] = append(
				[]crossFieldValidationRule(nil),
				operationExtension.crossFields...,
			)
		}
	}

	if err := catalog.compileParameters(label, operationID, pathItem, operation); err != nil {
		return err
	}
	if err := catalog.compileRequestBody(label, operationID, operation.RequestBody); err != nil {
		return err
	}
	return nil
}

func (catalog *ValidationCatalog) compileParameters(
	label string,
	operationID string,
	pathItem *openapi3.PathItem,
	operation *openapi3.Operation,
) error {
	parameters := make(map[string]*openapi3.Parameter)
	if pathItem != nil {
		for _, parameterRef := range pathItem.Parameters {
			if parameterRef == nil || parameterRef.Value == nil {
				continue
			}
			parameter := parameterRef.Value
			parameters[parameter.In+"\x00"+parameter.Name] = parameter
		}
	}
	for _, parameterRef := range operation.Parameters {
		if parameterRef == nil || parameterRef.Value == nil {
			continue
		}
		parameter := parameterRef.Value
		parameters[parameter.In+"\x00"+parameter.Name] = parameter
	}

	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parameter := parameters[key]
		location := parameterLocation(parameter.In)
		fieldLabel := fmt.Sprintf("%s %s.%s", label, location, parameter.Name)
		if parameter.Schema != nil {
			if err := catalog.compileField(operationID, location, parameter.Name, parameter.Schema, parameter.Required, fieldLabel); err != nil {
				return err
			}
		}
		if len(parameter.Content) > 0 {
			mediaTypes := make([]string, 0, len(parameter.Content))
			for mediaType := range parameter.Content {
				mediaTypes = append(mediaTypes, mediaType)
			}
			sort.Strings(mediaTypes)
			for _, mediaType := range mediaTypes {
				content := parameter.Content[mediaType]
				if content == nil || content.Schema == nil {
					continue
				}
				if err := catalog.compileField(operationID, location, parameter.Name, content.Schema, parameter.Required, fieldLabel); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (catalog *ValidationCatalog) compileRequestBody(label, operationID string, requestBodyRef *openapi3.RequestBodyRef) error {
	if requestBodyRef == nil || requestBodyRef.Value == nil {
		return nil
	}
	requestBody := requestBodyRef.Value
	mediaTypes := make([]string, 0, len(requestBody.Content))
	for mediaType := range requestBody.Content {
		mediaTypes = append(mediaTypes, mediaType)
	}
	sort.Strings(mediaTypes)
	for _, mediaType := range mediaTypes {
		content := requestBody.Content[mediaType]
		if content == nil || content.Schema == nil {
			continue
		}
		if err := catalog.compileBodySchema(label, operationID, content.Schema, nil, make(map[*openapi3.Schema]bool)); err != nil {
			return err
		}
	}
	return nil
}

func (catalog *ValidationCatalog) compileBodySchema(
	label string,
	operationID string,
	schemaRef *openapi3.SchemaRef,
	path []string,
	stack map[*openapi3.Schema]bool,
) error {
	if schemaRef == nil || schemaRef.Value == nil {
		return nil
	}
	schema := schemaRef.Value
	if stack[schema] {
		return nil
	}
	stack[schema] = true
	defer delete(stack, schema)

	for _, allOfRef := range schema.AllOf {
		if err := catalog.compileBodySchema(label, operationID, allOfRef, path, stack); err != nil {
			return err
		}
	}

	propertyNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propertyNames = append(propertyNames, name)
	}
	sort.Strings(propertyNames)
	for _, name := range propertyNames {
		propertyRef := schema.Properties[name]
		propertyPath := append(append([]string(nil), path...), name)
		field := strings.Join(propertyPath, ".")
		fieldLabel := fmt.Sprintf("%s %s.%s", label, requesterror.LocationBody, field)
		if err := catalog.compileField(
			operationID,
			requesterror.LocationBody,
			field,
			propertyRef,
			containsString(schema.Required, name),
			fieldLabel,
		); err != nil {
			return err
		}
		if err := catalog.compileBodySchema(label, operationID, propertyRef, propertyPath, stack); err != nil {
			return err
		}
	}
	return nil
}

func (catalog *ValidationCatalog) compileField(
	operationID string,
	location string,
	field string,
	schemaRef *openapi3.SchemaRef,
	required bool,
	label string,
) error {
	extension, present, err := parseFieldValidationExtension(schemaRef, label)
	if err != nil {
		return fmt.Errorf("compile validation catalog: %w", err)
	}
	if !present {
		return nil
	}
	if operationID == "" {
		return fmt.Errorf("compile validation catalog: %s requires operationId", label)
	}
	if schemaRef == nil || schemaRef.Value == nil {
		return fmt.Errorf("compile validation catalog: %s schema cannot be resolved", label)
	}

	for _, rule := range nativeSchemaValidationRules(schemaRef.Value, required) {
		if strings.TrimSpace(extension.messages[rule]) == "" {
			return fmt.Errorf("compile validation catalog: %s native rule %s must declare a non-empty messages.%s", label, rule, rule)
		}
	}
	for _, rule := range sortedStringKeys(extension.messages) {
		if err := catalog.addMessage(validationMessageKey{
			operationID: operationID,
			location:    location,
			field:       field,
			rule:        rule,
		}, extension.messages[rule]); err != nil {
			return fmt.Errorf("compile validation catalog: %s: %w", label, err)
		}
	}
	compiledCustomRules := make([]namedValidationRule, 0, len(extension.customRules)+1)
	if extension.notBlank {
		compiledCustomRules = append(compiledCustomRules, namedValidationRule{
			name:    customRuleNotBlank,
			message: extension.messages[customRuleNotBlank],
		})
	}
	compiledCustomRules = append(compiledCustomRules, extension.customRules...)
	if len(compiledCustomRules) > 0 {
		fieldKey := validationFieldKey{operationID: operationID, location: location, field: field}
		catalog.fieldRules[fieldKey] = append([]namedValidationRule(nil), compiledCustomRules...)
		for _, rule := range compiledCustomRules {
			if err := catalog.addMessage(validationMessageKey{
				operationID: operationID,
				location:    location,
				field:       field,
				rule:        rule.name,
			}, rule.message); err != nil {
				return fmt.Errorf("compile validation catalog: %s: %w", label, err)
			}
		}
	}
	return nil
}

func (catalog *ValidationCatalog) addMessage(key validationMessageKey, message string) error {
	key.operationID = strings.TrimSpace(key.operationID)
	key.location = strings.TrimSpace(key.location)
	key.field = strings.TrimSpace(key.field)
	key.rule = strings.TrimSpace(key.rule)
	if key.operationID == "" || key.rule == "" || strings.TrimSpace(message) == "" {
		return fmt.Errorf("validation message key and value must be non-empty")
	}
	if existing, ok := catalog.messages[key]; ok && existing != message {
		return fmt.Errorf(
			"conflicting validation messages for operationId=%s location=%s field=%s rule=%s",
			key.operationID,
			key.location,
			key.field,
			key.rule,
		)
	}
	catalog.messages[key] = message
	return nil
}

func (catalog *ValidationCatalog) message(operationID, location, field, rule string) (string, bool) {
	if catalog == nil {
		return "", false
	}
	key := validationMessageKey{
		operationID: strings.TrimSpace(operationID),
		location:    strings.TrimSpace(location),
		field:       strings.TrimSpace(field),
		rule:        strings.TrimSpace(rule),
	}
	if message, ok := catalog.messages[key]; ok {
		return message, true
	}
	message, ok := catalog.messages[validationMessageKey{
		operationID: key.operationID,
		rule:        key.rule,
	}]
	return message, ok
}

func nativeSchemaValidationRules(schema *openapi3.Schema, required bool) []string {
	if schema == nil {
		return nil
	}
	rules := make([]string, 0, 8)
	if required {
		rules = append(rules, "required")
	}
	if schema.MinLength > 0 {
		rules = append(rules, "minLength")
	}
	if schema.MaxLength != nil {
		rules = append(rules, "maxLength")
	}
	if strings.TrimSpace(schema.Pattern) != "" {
		rules = append(rules, "pattern")
	}
	if strings.TrimSpace(schema.Format) != "" {
		rules = append(rules, "format")
	}
	if len(schema.Enum) > 0 {
		rules = append(rules, "enum")
	}
	if schema.Min != nil {
		rules = append(rules, "minimum")
	}
	if schema.Max != nil {
		rules = append(rules, "maximum")
	}
	return rules
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
