package contract

import (
	"errors"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	"eventhub-go/internal/http/requesterror"
)

func parameterViolation(catalog *ValidationCatalog, requestErr *openapi3filter.RequestError) requesterror.Violation {
	parameter := requestErr.Parameter
	name := parameter.Name
	if name == "" {
		name = "parameter"
	}
	location := parameterLocation(parameter.In)
	rule := validationRule(requestErr.Err)
	message := parameterErrorMessage(parameter, requestErr)
	if catalogMessage, ok := catalog.message(validationOperationID(requestErr), location, name, rule); ok {
		message = catalogMessage
	}
	return requesterror.Violation{
		Location: location,
		Field:    name,
		Path:     name,
		Rule:     rule,
		Message:  message,
	}
}

func bodyFieldErrors(catalog *ValidationCatalog, requestErr *openapi3filter.RequestError) requesterror.Violations {
	if schemaErr, pointer := leafSchemaError(requestErr.Err); schemaErr != nil {
		field := "body"
		path := "body"
		if len(pointer) > 0 {
			field = pointer[0]
			path = strings.Join(pointer, ".")
		}
		rule := validationRule(schemaErr)
		message := schemaErr.Reason
		if message == "" {
			message = "字段不符合请求体 schema"
		}
		if catalogMessage, ok := catalog.message(
			validationOperationID(requestErr),
			requesterror.LocationBody,
			path,
			rule,
		); ok {
			message = catalogMessage
		}
		return requesterror.Violations{{
			Location: requesterror.LocationBody,
			Field:    field,
			Path:     path,
			Rule:     rule,
			Message:  message,
		}}
	}
	return requesterror.Violations{{
		Location: requesterror.LocationBody,
		Field:    "body",
		Path:     "body",
		Rule:     validationRule(requestErr.Err),
		Message:  "请求体不符合 OpenAPI schema",
	}}
}

func leafSchemaError(err error) (*openapi3.SchemaError, []string) {
	var (
		leaf    *openapi3.SchemaError
		pointer []string
	)
	for err != nil {
		var schemaErr *openapi3.SchemaError
		if !errors.As(err, &schemaErr) {
			break
		}
		leaf = schemaErr
		pointer = append(pointer, schemaErr.JSONPointer()...)
		if schemaErr.Origin == nil {
			break
		}
		err = schemaErr.Origin
	}
	return leaf, pointer
}

func validationOperationID(requestErr *openapi3filter.RequestError) string {
	if requestErr == nil || requestErr.Input == nil || requestErr.Input.Route == nil || requestErr.Input.Route.Operation == nil {
		return ""
	}
	return strings.TrimSpace(requestErr.Input.Route.Operation.OperationID)
}

func parameterLocation(location string) string {
	switch location {
	case openapi3.ParameterInPath:
		return requesterror.LocationPath
	case openapi3.ParameterInHeader:
		return requesterror.LocationHeader
	case openapi3.ParameterInCookie:
		return requesterror.LocationCookie
	default:
		return requesterror.LocationQuery
	}
}

func parameterErrorMessage(parameter *openapi3.Parameter, requestErr *openapi3filter.RequestError) string {
	switch parameter.In {
	case openapi3.ParameterInPath:
		return parameter.Name + " 不符合路径参数契约"
	case openapi3.ParameterInQuery:
		return parameter.Name + " 不符合查询参数契约"
	case openapi3.ParameterInHeader:
		return parameter.Name + " 不符合请求头契约"
	case openapi3.ParameterInCookie:
		return parameter.Name + " 不符合 Cookie 契约"
	default:
		if requestErr.Reason != "" {
			return requestErr.Reason
		}
		return "请求参数不符合 OpenAPI 契约"
	}
}

func validationRule(err error) string {
	if errors.Is(err, openapi3filter.ErrInvalidRequired) {
		return "required"
	}
	if errors.Is(err, openapi3filter.ErrInvalidEmptyValue) {
		return "allowEmptyValue"
	}
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) && schemaErr.SchemaField != "" {
		return schemaErr.SchemaField
	}
	var parseErr *openapi3filter.ParseError
	if errors.As(err, &parseErr) {
		return "type"
	}
	return "schema"
}
