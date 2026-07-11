package openapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestOpenAPIPolicy 把 api/openapi/eventhub.yaml 当作“可执行 API 规范”检查。
//
// 这类测试不启动 HTTP server，也不调用 handler；它只解析 OpenAPI 文档本身，
// 用 Go test 固化团队约定。这样后续新增接口时，如果忘记 operationId、tag、
// 统一响应 envelope、管理员角色声明等契约字段，会在本地测试和 CI 中尽早失败。
func TestOpenAPIPolicy(t *testing.T) {
	doc := loadOpenAPIDocument(t)
	operations := collectOperations(doc)

	// 顶层 security 会变成整份 spec 的默认认证要求。当前项目存在 login、register、
	// refresh 等公开接口，所以统一要求“每个 operation 自己声明安全策略”，避免公开
	// 接口被文档层误标成 BearerAuth。
	if len(doc.Security) != 0 {
		t.Errorf("top-level security must stay empty; declare auth per operation so public endpoints remain explicit")
	}

	assertBearerAuthSecurityScheme(t, doc)
	assertErrorResponseEnvelope(t, doc)
	assertErrorResponseViolations(t, doc)

	for _, item := range operations {
		assertOperationMetadata(t, item)

		// Actuator 端点服务健康检查和基础探针，契约上允许不使用业务 ApiResponse envelope。
		// 其他接口都按 EventHub 业务 API 规范检查 JSON 响应、成功 envelope 和错误响应引用。
		if strings.HasPrefix(item.path, "/actuator/") {
			continue
		}

		assertBusinessJSONResponses(t, doc, item)
		assertBusinessSuccessEnvelope(t, doc, item)
		assertCentralizedErrorResponses(t, doc, item)

		if strings.HasPrefix(item.path, "/api/v1/admin/") {
			assertAdminSecurityPolicy(t, item)
		} else {
			assertNonAdminRolePolicy(t, item)
		}
	}

	assertAuthSecurityPolicy(t, operations)
}

// TestOpenAPIValidationPolicy 固化 x-validation 的声明格式。
//
// policy 扫描 operation request body 的顶层字段和 query 参数，固定 native/custom rule
// 的声明与消息形态，避免把响应模型上的 schema 约束误当成请求校验契约。
func TestOpenAPIValidationPolicy(t *testing.T) {
	doc := loadOpenAPIDocument(t)
	operations := collectOperations(doc)

	for _, field := range collectRequestValidationFields(operations) {
		field := field
		t.Run(field.label, func(t *testing.T) {
			for _, violation := range fieldValidationPolicyViolations(field) {
				t.Errorf("%s", violation)
			}
		})
	}

	for _, item := range operations {
		rawValidation, ok := item.operation.Extensions["x-validation"]
		if !ok {
			continue
		}
		validation, ok := rawValidation.(map[string]any)
		if !ok {
			t.Errorf("%s x-validation must be an object", item.label())
			continue
		}
		for key := range validation {
			switch key {
			case "crossFields", "messages":
			default:
				t.Errorf("%s x-validation.%s is not allowed at operation level", item.label(), key)
			}
		}
		if _, ok := validation["crossFields"]; !ok {
			continue
		}
		for _, violation := range crossFieldValidationPolicyViolations(item) {
			t.Errorf("%s", violation)
		}
	}
}

// TestOpenAPIJavaParityValidationMessagesAreStable 固化当前 Java DTO 的字段消息。
//
// 仅检查“存在 message”不足以防止文案漂移；这里对当前已迁移请求字段逐条断言准确中文消息，
// 并覆盖 Go OpenAPI 已声明的时间格式、custom rule 和枚举规则。
func TestOpenAPIJavaParityValidationMessagesAreStable(t *testing.T) {
	doc := loadOpenAPIDocument(t)
	operations := collectOperations(doc)
	fieldsByLabel := make(map[string]validationField)
	for _, field := range collectRequestValidationFields(operations) {
		fieldsByLabel[field.label] = field
	}

	expectedMessages := javaParityValidationMessages()
	labels := make([]string, 0, len(expectedMessages))
	for label := range expectedMessages {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	for _, label := range labels {
		field, ok := fieldsByLabel[label]
		if !ok {
			t.Errorf("Java parity validation field %s must exist", label)
			continue
		}
		validation, ok := validationExtension(field.schemaRef)
		if !ok {
			t.Errorf("%s must declare x-validation", label)
			continue
		}
		messages, ok := stringMap(validation["messages"])
		if !ok {
			t.Errorf("%s x-validation.messages must be a string map", label)
			continue
		}

		rules := make([]string, 0, len(expectedMessages[label]))
		for rule := range expectedMessages[label] {
			rules = append(rules, rule)
		}
		sort.Strings(rules)
		for _, rule := range rules {
			want := expectedMessages[label][rule]
			got := strings.TrimSpace(messages[rule])
			if got != want {
				t.Errorf("%s messages.%s: got %q want %q", label, rule, got, want)
			}
		}
	}

	assertJavaParityCustomRules(t, fieldsByLabel)
	assertJavaParityCrossFieldRules(t, operations)
}

// TestOpenAPIGeneratedFilesAreSplit 固化 oapi-codegen 生成物的可读性边界。
//
// HTTP 层仍统一 import eventhub-go/api/openapi/gen；这里仅约束物理文件布局：
// models.gen.go 放 schema/request/response model，server.gen.go 放 chi/strict server wrapper。
// 旧 eventhub.gen.go 不能残留，否则会和拆分后的同 package 类型重复声明。
func TestOpenAPIGeneratedFilesAreSplit(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate OpenAPI policy test file")
	}

	genDir := filepath.Join(filepath.Dir(currentFile), "gen")
	for _, name := range []string{"models.gen.go", "server.gen.go"} {
		path := filepath.Join(genDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}

	legacyFile := filepath.Join(genDir, "eventhub.gen.go")
	if _, err := os.Stat(legacyFile); err == nil {
		t.Fatalf("legacy generated file %s must be removed after splitting OpenAPI generated output", legacyFile)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy generated file %s: %v", legacyFile, err)
	}
}

// TestSchemaUsesTopLevelComponentRequiresEnvelope 固化统一响应 envelope 的“顶层”判断口径。
//
// OpenAPI 中成功响应可能直接 `$ref` 到 ApiResponse，也可能先 `$ref` 到某个具名响应
// schema，再由该 schema 通过顶层 allOf 组合 ApiResponse。两种写法都代表 HTTP 响应
// 最外层是统一 envelope，应当被接受。
//
// 反过来，如果 ApiResponse 只出现在 properties 里的某个嵌套字段，说明统一响应信封
// 被放进了 payload 内部，而不是作为 HTTP 响应最外层结构。该测试专门覆盖这个误判点，
// 防止后续扩展 schemaUsesTopLevelComponent 时把“任意子树出现 ApiResponse”当成合格。
func TestSchemaUsesTopLevelComponentRequiresEnvelope(t *testing.T) {
	apiResponseRef := openapi3.NewSchemaRef("#/components/schemas/ApiResponse", nil)
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"ApiResponse": apiResponseRef,
				"ApiResponseUserInfo": openapi3.NewSchemaRef("", &openapi3.Schema{
					AllOf: openapi3.SchemaRefs{
						apiResponseRef,
						openapi3.NewSchemaRef("", openapi3.NewObjectSchema().WithPropertyRef("data", openapi3.NewSchemaRef("#/components/schemas/UserInfo", nil))),
					},
				}),
				"NestedOnlyResponse": openapi3.NewSchemaRef("", openapi3.NewObjectSchema().WithPropertyRef("payload", apiResponseRef)),
				"UserInfo":           openapi3.NewSchemaRef("", openapi3.NewObjectSchema()),
			},
		},
	}

	tests := []struct {
		name   string
		schema *openapi3.SchemaRef
		want   bool
	}{
		{
			name:   "direct ApiResponse ref is an envelope",
			schema: apiResponseRef,
			want:   true,
		},
		{
			name:   "component allOf composed with ApiResponse is an envelope",
			schema: openapi3.NewSchemaRef("#/components/schemas/ApiResponseUserInfo", nil),
			want:   true,
		},
		{
			name:   "nested ApiResponse property is not a top-level envelope",
			schema: openapi3.NewSchemaRef("#/components/schemas/NestedOnlyResponse", nil),
			want:   false,
		},
		{
			name:   "inline property with ApiResponse is not a top-level envelope",
			schema: openapi3.NewSchemaRef("", openapi3.NewObjectSchema().WithPropertyRef("payload", apiResponseRef)),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 本质上在验证 schemaUsesTopLevelComponent 方法的正确性
			got := schemaUsesTopLevelComponent(doc, tt.schema, "ApiResponse", map[*openapi3.SchemaRef]bool{})
			if got != tt.want {
				t.Fatalf("schemaUsesTopLevelComponent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCentralizedErrorResponsesRejectsComponentSchemaDrift 验证组件错误响应不能漂移。
//
// operation 级检查只能证明非 2xx 响应引用了 components.responses.BadRequest 这类集中定义，
// 不能证明 BadRequest 组件内部仍然是统一错误响应。这里构造一个反例：operation 的 400
// 响应仍然引用 components.responses.BadRequest，但 BadRequest 的 schema 故意指向
// ApiResponseVoid。期望 helper 返回违规信息，证明 policy 能继续检查组件内部 schema。
func TestCentralizedErrorResponsesRejectsComponentSchemaDrift(t *testing.T) {
	apiResponseRef := openapi3.NewSchemaRef("#/components/schemas/ApiResponse", nil)
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Responses: openapi3.ResponseBodies{
				"BadRequest": {
					Value: openapi3.NewResponse().
						WithJSONSchemaRef(openapi3.NewSchemaRef("#/components/schemas/ApiResponseVoid", nil)),
				},
			},
			Schemas: openapi3.Schemas{
				"ApiResponse": apiResponseRef,
				"ApiResponseVoid": openapi3.NewSchemaRef("", &openapi3.Schema{
					AllOf: openapi3.SchemaRefs{
						apiResponseRef,
						openapi3.NewSchemaRef("", openapi3.NewObjectSchema()),
					},
				}),
				"ErrorResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					AllOf: openapi3.SchemaRefs{
						apiResponseRef,
						openapi3.NewSchemaRef("", openapi3.NewObjectSchema()),
					},
				}),
			},
		},
	}
	item := operationItem{
		method: "GET",
		path:   "/api/v1/example",
		operation: &openapi3.Operation{
			OperationID: "example",
			Responses: openapi3.NewResponses(openapi3.WithStatus(400,
				&openapi3.ResponseRef{Ref: "#/components/responses/BadRequest"})),
		},
	}

	got := componentErrorResponseViolation(doc, item, "400", "BadRequest")
	if got == "" {
		t.Fatalf("componentErrorResponseViolation should reject component responses that do not use ErrorResponse")
	}
	if !strings.Contains(got, "must use ErrorResponse as the top-level error schema") {
		t.Fatalf("componentErrorResponseViolation() = %q", got)
	}
}

// TestErrorResponseSchemaRequiresApiResponseEnvelope 验证 ErrorResponse 自身必须复用 ApiResponse。
//
// 即使所有 components.responses 都引用 ErrorResponse，如果 ErrorResponse 被改成普通 object，
// 统一错误体仍会丢失 code/message/data/requestId/timestamp 这组对外契约字段。该测试用
// 一个没有 allOf ApiResponse 的 ErrorResponse 作为反例，确保全局错误 schema 漂移会被
// policy helper 捕获。
func TestErrorResponseSchemaRequiresApiResponseEnvelope(t *testing.T) {
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"ApiResponse":   openapi3.NewSchemaRef("", openapi3.NewObjectSchema()),
				"ErrorResponse": openapi3.NewSchemaRef("", openapi3.NewObjectSchema()),
			},
		},
	}

	got := errorResponseEnvelopeViolation(doc)
	if got == "" {
		t.Fatalf("errorResponseEnvelopeViolation should reject ErrorResponse schemas that do not use ApiResponse")
	}
	if !strings.Contains(got, "must use ApiResponse as the top-level envelope") {
		t.Fatalf("errorResponseEnvelopeViolation() = %q", got)
	}
}

// TestErrorResponseSchemaRequiresViolationsContract 验证错误详情不能退回松散字段 map。
func TestErrorResponseSchemaRequiresViolationsContract(t *testing.T) {
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"ErrorResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Properties: openapi3.Schemas{
						"data": openapi3.NewSchemaRef("", openapi3.NewObjectSchema()),
					},
				}),
			},
		},
	}

	got := errorResponseViolationsViolation(doc)
	if got == "" {
		t.Fatal("errorResponseViolationsViolation should reject ErrorResponse.data without violations")
	}
	if !strings.Contains(got, "violations") {
		t.Fatalf("errorResponseViolationsViolation() = %q", got)
	}
}

// TestErrorResponseViolationsMustStayOptional 防止字段错误结构误收窄全局 ErrorResponse.data。
func TestErrorResponseViolationsMustStayOptional(t *testing.T) {
	violationSchema := openapi3.NewObjectSchema().WithoutAdditionalProperties()
	for _, field := range []string{"location", "field", "path", "rule", "message"} {
		violationSchema.WithProperty(field, openapi3.NewStringSchema())
		violationSchema.Required = append(violationSchema.Required, field)
	}
	violationsSchema := openapi3.NewArraySchema()
	violationsSchema.Items = openapi3.NewSchemaRef("#/components/schemas/Violation", violationSchema)
	dataSchema := openapi3.NewObjectSchema().WithNullable().WithAnyAdditionalProperties()
	dataSchema.WithProperty("violations", violationsSchema)
	dataSchema.Required = []string{"violations"}
	doc := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"ErrorResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Properties: openapi3.Schemas{
						"data": openapi3.NewSchemaRef("", dataSchema),
					},
				}),
				"Violation": openapi3.NewSchemaRef("", violationSchema),
			},
		},
	}

	got := errorResponseViolationsViolation(doc)
	if !strings.Contains(got, "must remain optional") {
		t.Fatalf("errorResponseViolationsViolation() = %q", got)
	}
}

// TestAdminSecurityPolicyRequiresExactAdminRole 验证管理员接口角色声明必须精确。
//
// 仅检查“包含 ADMIN”会允许 `x-required-roles: [ADMIN, USER]` 之类漂移通过，
// 文档读者和后续治理工具就无法判断真实授权边界是否仍然只有 ADMIN。
func TestAdminSecurityPolicyRequiresExactAdminRole(t *testing.T) {
	item := operationItem{
		method: "GET",
		path:   "/api/v1/admin/users",
		operation: &openapi3.Operation{
			OperationID: "listAdminUsers",
			Security: &openapi3.SecurityRequirements{
				openapi3.SecurityRequirement{"BearerAuth": []string{}},
			},
			Extensions: map[string]any{
				"x-required-roles": []any{"ADMIN", "USER"},
			},
		},
	}

	got := adminSecurityPolicyViolation(item)
	if got == "" {
		t.Fatalf("adminSecurityPolicyViolation should reject admin operations whose roles are not exactly [ADMIN]")
	}
	if !strings.Contains(got, "x-required-roles must equal [ADMIN]") {
		t.Fatalf("adminSecurityPolicyViolation() = %q", got)
	}
}

// TestNonAdminOperationMustNotDeclareAdminRole 防止非管理端接口误标 ADMIN。
//
// ADMIN 角色是管理端边界，不应因为复制粘贴 OpenAPI operation 时把普通接口也标记为
// 管理员接口；如果未来确有非 /api/v1/admin/** 的 ADMIN-only 能力，应先更新设计和 policy。
func TestNonAdminOperationMustNotDeclareAdminRole(t *testing.T) {
	item := operationItem{
		method: "GET",
		path:   "/api/v1/me",
		operation: &openapi3.Operation{
			OperationID: "getCurrentUser",
			Extensions: map[string]any{
				"x-required-roles": []any{"ADMIN"},
			},
		},
	}

	got := adminRoleLeakViolation(item)
	if got == "" {
		t.Fatalf("adminRoleLeakViolation should reject non-admin operations that declare ADMIN")
	}
	if !strings.Contains(got, "non-admin operation must not declare ADMIN") {
		t.Fatalf("adminRoleLeakViolation() = %q", got)
	}
}

// operationItem 是测试内部的规范检查单元。
//
// kin-openapi 会按 path -> method -> operation 组织文档；测试失败时只拿 operation
// 本身不够定位问题，所以这里同时保留 method 和 path，用于生成稳定、可读的失败信息。
type operationItem struct {
	method    string
	path      string
	operation *openapi3.Operation
}

// routeKey 使用 method + path 作为 operation 的唯一定位键。
func (item operationItem) routeKey() string {
	return fmt.Sprintf("%s %s", item.method, item.path)
}

// label 统一生成失败信息前缀，确保每个错误都能定位到 method/path/operationId。
func (item operationItem) label() string {
	operationID := strings.TrimSpace(item.operation.OperationID)
	if operationID == "" {
		operationID = "<missing>"
	}
	return fmt.Sprintf("%s (operationId=%s)", item.routeKey(), operationID)
}

// validationField 是 policy test 对请求字段的最小抽象。
// required 来自父 object.required 或 parameter.required；其他规则来自 schemaRef.Value。
type validationField struct {
	label     string
	required  bool
	schemaRef *openapi3.SchemaRef
}

// crossFieldRule 表示 operation 级 x-validation.crossFields 的固定声明形态。
type crossFieldRule struct {
	name    string
	rule    string
	left    string
	right   string
	message string
}

// collectRequestValidationFields 只收集当前 HTTP 请求边界的 body 顶层字段和 query 参数。
//
// request body 的 required 信息位于父 object schema；query required 位于 Parameter。
// 跳过 response schema 和 path/header/cookie 是阶段 1 的刻意范围约束。
func collectRequestValidationFields(operations []operationItem) []validationField {
	var fields []validationField

	for _, item := range operations {
		var requestBody *openapi3.RequestBody
		if item.operation.RequestBody != nil {
			requestBody = item.operation.RequestBody.Value
		}
		if requestBody != nil {
			mediaType := requestBody.Content["application/json"]
			if mediaType != nil && mediaType.Schema != nil {
				bodySchema := mediaType.Schema.Value
				if bodySchema != nil {
					propertyNames := make([]string, 0, len(bodySchema.Properties))
					for name := range bodySchema.Properties {
						propertyNames = append(propertyNames, name)
					}
					sort.Strings(propertyNames)
					for _, name := range propertyNames {
						fields = append(fields, validationField{
							label:     fmt.Sprintf("%s body.%s", item.routeKey(), name),
							required:  contains(bodySchema.Required, name),
							schemaRef: bodySchema.Properties[name],
						})
					}
				}
			}
		}

		for _, parameterRef := range item.operation.Parameters {
			var parameter *openapi3.Parameter
			if parameterRef != nil {
				parameter = parameterRef.Value
			}
			if parameter == nil || parameter.In != openapi3.ParameterInQuery {
				continue
			}
			fields = append(fields, validationField{
				label:     fmt.Sprintf("%s query.%s", item.routeKey(), parameter.Name),
				required:  parameter.Required,
				schemaRef: parameter.Schema,
			})
		}
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].label < fields[j].label
	})
	return fields
}

// fieldValidationPolicyViolations 返回字段级 x-validation 的全部格式违规。
func fieldValidationPolicyViolations(field validationField) []string {
	var schema *openapi3.Schema
	if field.schemaRef != nil {
		schema = field.schemaRef.Value
	}
	if schema == nil {
		return []string{fmt.Sprintf("%s schema cannot be resolved", field.label)}
	}

	ruleNames := schemaValidationRuleNames(schema, field.required)
	rawValidation, hasValidation := rawValidationExtension(field.schemaRef)
	if !hasValidation {
		if len(ruleNames) == 0 {
			return nil
		}
		return []string{fmt.Sprintf("%s has field rules %v and must declare x-validation messages", field.label, ruleNames)}
	}

	validation, ok := rawValidation.(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("%s x-validation must be an object", field.label)}
	}

	var violations []string
	for key := range validation {
		switch key {
		case "messages", "notBlank", "rules":
		default:
			violations = append(violations, fmt.Sprintf("%s x-validation.%s is not allowed at field level", field.label, key))
		}
	}
	if rawNotBlank, exists := validation["notBlank"]; exists {
		notBlank, ok := rawNotBlank.(bool)
		if !ok || !notBlank {
			violations = append(violations, fmt.Sprintf("%s x-validation.notBlank must be true when declared", field.label))
		} else {
			ruleNames = append(ruleNames, "notBlank")
		}
	}

	messages, messagesOK := stringMap(validation["messages"])
	if len(ruleNames) > 0 && !messagesOK {
		violations = append(violations, fmt.Sprintf("%s x-validation.messages must be a string map", field.label))
	} else {
		for _, ruleName := range ruleNames {
			if strings.TrimSpace(messages[ruleName]) == "" {
				violations = append(violations, fmt.Sprintf("%s field rule %s must declare a non-empty messages.%s", field.label, ruleName, ruleName))
			}
		}
	}

	if rawRules, exists := validation["rules"]; exists {
		rules, ok := rawRules.([]any)
		if !ok || len(rules) == 0 {
			violations = append(violations, fmt.Sprintf("%s x-validation.rules must be a non-empty array", field.label))
		} else {
			for index, rawRule := range rules {
				rule, ok := rawRule.(map[string]any)
				if !ok {
					violations = append(violations, fmt.Sprintf("%s x-validation.rules[%d] must be an object", field.label, index))
					continue
				}
				name, nameOK := nonEmptyString(rule["name"])
				_, messageOK := nonEmptyString(rule["message"])
				if !nameOK {
					violations = append(violations, fmt.Sprintf("%s x-validation.rules[%d].name must be a non-empty string", field.label, index))
				}
				if !messageOK {
					violations = append(violations, fmt.Sprintf("%s x-validation.rules[%d].message must be a non-empty string", field.label, index))
				}
				if nameOK && name != "containsLetterAndDigit" && name != "localDateTime" {
					violations = append(violations, fmt.Sprintf("%s x-validation.rules[%d] uses unsupported custom rule %q", field.label, index, name))
				}
			}
		}
	}

	return violations
}

// crossFieldValidationPolicyViolations 校验 operation 级 crossFields 格式和规则白名单。
func crossFieldValidationPolicyViolations(item operationItem) []string {
	rawValidation := item.operation.Extensions["x-validation"]
	validation, ok := rawValidation.(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("%s x-validation must be an object", item.label())}
	}

	var violations []string

	rawCrossFields, ok := validation["crossFields"]
	if !ok {
		return append(violations, fmt.Sprintf("%s x-validation.crossFields must be declared", item.label()))
	}
	crossFields, ok := rawCrossFields.([]any)
	if !ok || len(crossFields) == 0 {
		return append(violations, fmt.Sprintf("%s x-validation.crossFields must be a non-empty array", item.label()))
	}

	requiredKeys := []string{"name", "rule", "left", "right", "message"}
	for index, rawCrossField := range crossFields {
		crossField, ok := rawCrossField.(map[string]any)
		if !ok {
			violations = append(violations, fmt.Sprintf("%s x-validation.crossFields[%d] must be an object", item.label(), index))
			continue
		}
		for _, key := range requiredKeys {
			if _, ok := nonEmptyString(crossField[key]); !ok {
				violations = append(violations, fmt.Sprintf("%s x-validation.crossFields[%d].%s must be a non-empty string", item.label(), index, key))
			}
		}
		if rule, ok := nonEmptyString(crossField["rule"]); ok && rule != "notAfter" {
			violations = append(violations, fmt.Sprintf("%s x-validation.crossFields[%d] uses unsupported custom rule %q", item.label(), index, rule))
		}
	}

	return violations
}

// schemaValidationRuleNames 返回需要稳定 message 的 OpenAPI 原生字段规则。
func schemaValidationRuleNames(schema *openapi3.Schema, required bool) []string {
	var rules []string
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

// rawValidationExtension 同时兼容内联 Schema.Extensions 和 `$ref` 旁的 SchemaRef.Extensions。
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

func validationExtension(schemaRef *openapi3.SchemaRef) (map[string]any, bool) {
	raw, ok := rawValidationExtension(schemaRef)
	if !ok {
		return nil, false
	}
	validation, ok := raw.(map[string]any)
	return validation, ok
}

func stringMap(raw any) (map[string]string, bool) {
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	result := make(map[string]string, len(values))
	for key, rawValue := range values {
		value, ok := rawValue.(string)
		if !ok {
			return nil, false
		}
		result[key] = value
	}
	return result, true
}

func nonEmptyString(raw any) (string, bool) {
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, value != ""
}

// javaParityValidationMessages 返回当前 Java DTO 与既有 Go schema 的稳定消息基线。
// 时间字段没有 Java Bean Validation message，因此沿用 Go OpenAPI contract 的字段级格式提示；
// UpdateUserStatus.status 的 enum 消息复用 Java 管理员查询状态的允许值提示。
func javaParityValidationMessages() map[string]map[string]string {
	return map[string]map[string]string{
		"POST /api/v1/auth/register body.username": {
			"required":  "username 不能为空",
			"notBlank":  "username 不能为空",
			"minLength": "username 长度必须在 3 到 32 个字符之间",
			"maxLength": "username 长度必须在 3 到 32 个字符之间",
			"pattern":   "username 只能包含字母、数字和下划线",
		},
		"POST /api/v1/auth/register body.email": {
			"required":  "email 不能为空",
			"notBlank":  "email 不能为空",
			"format":    "email 格式不合法",
			"maxLength": "email 长度不能超过 128 个字符",
		},
		"POST /api/v1/auth/register body.password": {
			"required":  "password 不能为空",
			"notBlank":  "password 不能为空",
			"minLength": "password 长度必须在 8 到 72 个字符之间",
			"maxLength": "password 长度必须在 8 到 72 个字符之间",
		},
		"POST /api/v1/auth/login body.usernameOrEmail": {
			"required":  "用户名或邮箱不能为空",
			"notBlank":  "用户名或邮箱不能为空",
			"maxLength": "用户名或邮箱长度不能超过 128 个字符",
		},
		"POST /api/v1/auth/login body.password": {
			"required":  "密码不能为空",
			"notBlank":  "密码不能为空",
			"maxLength": "密码长度不能超过 72 个字符",
		},
		"POST /api/v1/auth/refresh body.refreshToken": {
			"required":  "refreshToken 不能为空",
			"notBlank":  "refreshToken 不能为空",
			"maxLength": "refreshToken 长度不能超过 128 个字符",
		},
		"PATCH /api/v1/admin/users/{userId}/status body.status": {
			"required": "status 不能为空",
			"enum":     "用户状态只能是 ENABLED 或 DISABLED",
		},
		"POST /api/v1/system/echo body.message": {
			"required":  "message 不能为空",
			"notBlank":  "message 不能为空",
			"minLength": "message 不能为空",
			"maxLength": "message 长度不能超过 64",
		},
		"POST /api/v1/system/echo body.tag": {
			"maxLength": "tag 长度不能超过 32",
		},
		"GET /api/v1/admin/users query.page": {
			"minimum": "页码不能小于 1",
		},
		"GET /api/v1/admin/users query.size": {
			"minimum": "每页条数不能小于 1",
			"maximum": "每页条数不能超过 100",
		},
		"GET /api/v1/admin/users query.username": {
			"maxLength": "用户名筛选长度不能超过 32",
		},
		"GET /api/v1/admin/users query.email": {
			"maxLength": "邮箱筛选长度不能超过 128",
		},
		"GET /api/v1/admin/users query.status": {
			"enum": "用户状态只能是 ENABLED 或 DISABLED",
		},
		"GET /api/v1/admin/users query.createdAtFrom": {
			"pattern": "createdAtFrom 格式不合法",
		},
		"GET /api/v1/admin/users query.createdAtTo": {
			"pattern": "createdAtTo 格式不合法",
		},
		"GET /api/v1/admin/users query.updatedAtFrom": {
			"pattern": "updatedAtFrom 格式不合法",
		},
		"GET /api/v1/admin/users query.updatedAtTo": {
			"pattern": "updatedAtTo 格式不合法",
		},
	}
}

func assertJavaParityCustomRules(t *testing.T, fieldsByLabel map[string]validationField) {
	t.Helper()

	const label = "POST /api/v1/auth/register body.password"
	field, ok := fieldsByLabel[label]
	if !ok {
		t.Errorf("Java parity validation field %s must exist", label)
		return
	}
	validation, ok := validationExtension(field.schemaRef)
	if !ok {
		t.Errorf("%s must declare x-validation", label)
		return
	}
	rules, ok := customRuleMessages(validation["rules"])
	if !ok {
		t.Errorf("%s x-validation.rules must contain named rules with messages", label)
		return
	}
	if len(rules) != 1 {
		t.Errorf("%s x-validation.rules: got %d rules want 1", label, len(rules))
	}
	const want = "password 至少包含字母和数字"
	got := strings.TrimSpace(rules["containsLetterAndDigit"])
	if got != want {
		t.Errorf("%s custom rule containsLetterAndDigit: got %q want %q", label, got, want)
	}

	localDateTimeMessages := map[string]string{
		"GET /api/v1/admin/users query.createdAtFrom": "createdAtFrom 格式不合法",
		"GET /api/v1/admin/users query.createdAtTo":   "createdAtTo 格式不合法",
		"GET /api/v1/admin/users query.updatedAtFrom": "updatedAtFrom 格式不合法",
		"GET /api/v1/admin/users query.updatedAtTo":   "updatedAtTo 格式不合法",
	}
	for fieldLabel, wantMessage := range localDateTimeMessages {
		field, ok := fieldsByLabel[fieldLabel]
		if !ok {
			t.Errorf("Java parity validation field %s must exist", fieldLabel)
			continue
		}
		validation, ok := validationExtension(field.schemaRef)
		if !ok {
			t.Errorf("%s must declare x-validation", fieldLabel)
			continue
		}
		rules, ok := customRuleMessages(validation["rules"])
		if !ok {
			t.Errorf("%s x-validation.rules must contain named rules with messages", fieldLabel)
			continue
		}
		if len(rules) != 1 {
			t.Errorf("%s x-validation.rules: got %d rules want 1", fieldLabel, len(rules))
		}
		if gotMessage := strings.TrimSpace(rules["localDateTime"]); gotMessage != wantMessage {
			t.Errorf("%s custom rule localDateTime: got %q want %q", fieldLabel, gotMessage, wantMessage)
		}
	}
}

func assertJavaParityCrossFieldRules(t *testing.T, operations []operationItem) {
	t.Helper()

	var target *operationItem
	for index := range operations {
		if operations[index].routeKey() == "GET /api/v1/admin/users" {
			target = &operations[index]
			break
		}
	}
	if target == nil {
		t.Errorf("GET /api/v1/admin/users must exist for Java parity validation")
		return
	}

	rules, ok := crossFieldRules(*target)
	if !ok {
		t.Errorf("%s must declare valid x-validation.crossFields", target.label())
		return
	}
	if len(rules) != 2 {
		t.Errorf("%s x-validation.crossFields: got %d rules want 2", target.label(), len(rules))
	}
	rulesByName := make(map[string]crossFieldRule, len(rules))
	for _, rule := range rules {
		rulesByName[rule.name] = rule
	}

	expected := []crossFieldRule{
		{
			name:    "createdAtRange",
			rule:    "notAfter",
			left:    "createdAtFrom",
			right:   "createdAtTo",
			message: "createdAtFrom 不能晚于 createdAtTo",
		},
		{
			name:    "updatedAtRange",
			rule:    "notAfter",
			left:    "updatedAtFrom",
			right:   "updatedAtTo",
			message: "updatedAtFrom 不能晚于 updatedAtTo",
		},
	}
	for _, want := range expected {
		got, ok := rulesByName[want.name]
		if !ok {
			t.Errorf("%s x-validation.crossFields must contain %s", target.label(), want.name)
			continue
		}
		if got.rule != want.rule || got.left != want.left || got.right != want.right || got.message != want.message {
			t.Errorf("%s crossFields.%s: got %+v want %+v", target.label(), want.name, got, want)
		}
	}
}

func customRuleMessages(raw any) (map[string]string, bool) {
	rawRules, ok := raw.([]any)
	if !ok || len(rawRules) == 0 {
		return nil, false
	}
	rules := make(map[string]string, len(rawRules))
	for _, rawRule := range rawRules {
		rule, ok := rawRule.(map[string]any)
		if !ok {
			return nil, false
		}
		name, nameOK := nonEmptyString(rule["name"])
		message, messageOK := nonEmptyString(rule["message"])
		if !nameOK || !messageOK {
			return nil, false
		}
		rules[name] = message
	}
	return rules, true
}

func crossFieldRules(item operationItem) ([]crossFieldRule, bool) {
	rawValidation, ok := item.operation.Extensions["x-validation"]
	if !ok {
		return nil, false
	}
	validation, ok := rawValidation.(map[string]any)
	if !ok {
		return nil, false
	}
	rawRules, ok := validation["crossFields"].([]any)
	if !ok || len(rawRules) == 0 {
		return nil, false
	}

	rules := make([]crossFieldRule, 0, len(rawRules))
	for _, rawRule := range rawRules {
		rule, ok := rawRule.(map[string]any)
		if !ok {
			return nil, false
		}
		name, nameOK := nonEmptyString(rule["name"])
		ruleName, ruleOK := nonEmptyString(rule["rule"])
		left, leftOK := nonEmptyString(rule["left"])
		right, rightOK := nonEmptyString(rule["right"])
		message, messageOK := nonEmptyString(rule["message"])
		if !nameOK || !ruleOK || !leftOK || !rightOK || !messageOK {
			return nil, false
		}
		rules = append(rules, crossFieldRule{
			name:    name,
			rule:    ruleName,
			left:    left,
			right:   right,
			message: message,
		})
	}
	return rules, true
}

// loadOpenAPIDocument 从当前测试文件所在目录加载 eventhub.yaml。
//
// 使用 runtime.Caller 动态获取 spec 文件绝对路径。不使用相对路径 ./eventhub.yaml 的原因是：
// 工作目录可能会因为测试执行方式不同（比如在根目录执行 go test ./... 还是在子目录执行）而发生变化。
// doc.Validate 会先执行 OpenAPI 结构校验；本文件后续断言再检查团队自定义规范。
func loadOpenAPIDocument(t *testing.T) *openapi3.T {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate OpenAPI policy test file")
	}
	specPath := filepath.Join(filepath.Dir(currentFile), "eventhub.yaml")

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load %s: %v", specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate %s: %v", specPath, err)
	}
	return doc
}

// collectOperations 展平 spec 中所有 path/method operation，并排序保证失败输出稳定。
//
// OpenAPI 的 Paths 底层是 map；不排序时 Go 的 map 遍历顺序不稳定，可能导致同一批
// 失败在不同机器上顺序不同，影响阅读和定位。
func collectOperations(doc *openapi3.T) []operationItem {
	var operations []operationItem
	for path, pathItem := range doc.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			operations = append(operations, operationItem{
				method:    strings.ToUpper(method),
				path:      path,
				operation: operation,
			})
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		left, right := operations[i], operations[j]
		if left.path != right.path {
			return left.path < right.path
		}
		return left.method < right.method
	})
	return operations
}

// assertOperationMetadata 检查每个 operation 都具备最基本的可维护性字段。
//
// operationId 用于生成代码和跨语言对齐；summary/description 帮助人阅读 API 意图；
// tag 则保证 Swagger UI、生成文档和后续按模块扫描时不会把接口混在一起。
func assertOperationMetadata(t *testing.T, item operationItem) {
	t.Helper()

	if strings.TrimSpace(item.operation.OperationID) == "" {
		t.Errorf("%s must declare operationId", item.label())
	}
	if strings.TrimSpace(item.operation.Summary) == "" && strings.TrimSpace(item.operation.Description) == "" {
		t.Errorf("%s must declare summary or description", item.label())
	}
	if len(item.operation.Tags) == 0 {
		t.Errorf("%s must declare at least one tag", item.label())
	}
}

// assertBusinessJSONResponses 要求业务接口的每个响应都声明 application/json。
//
// 这里检查所有状态码，而不只检查 2xx：成功和失败都要有稳定 JSON 契约。错误响应
// 是否集中引用 components.responses 由 assertCentralizedErrorResponses 进一步检查。
func assertBusinessJSONResponses(t *testing.T, doc *openapi3.T, item operationItem) {
	t.Helper()

	for status, responseRef := range item.operation.Responses.Map() {
		response := resolveResponse(doc, responseRef)
		if response == nil {
			t.Errorf("%s response %s cannot be resolved", item.label(), status)
			continue
		}
		if _, ok := response.Content["application/json"]; !ok {
			t.Errorf("%s response %s must declare application/json content", item.label(), status)
		}
	}
}

// assertBusinessSuccessEnvelope 要求非 actuator 的 2xx JSON 响应包在 ApiResponse 中。
//
// 当前业务约定是统一响应信封：即使 data 是分页对象 PageResponse / PageResponseUserInfo，
// 最外层仍应是 ApiResponse。OpenAPI 中常见写法是 allOf 组合“ApiResponse + 具体 data
// schema”，所以这里接受直接引用 ApiResponse 或最外层 allOf 组合 ApiResponse。
//
// 是否缺少 application/json 由 assertBusinessJSONResponses 报错；这里额外处理
// "application/json" key 存在但 media type 定义为空的异常结构，避免静默跳过或继续读取。
func assertBusinessSuccessEnvelope(t *testing.T, doc *openapi3.T, item operationItem) {
	t.Helper()

	for status, responseRef := range item.operation.Responses.Map() {
		if !is2xxStatus(status) {
			continue
		}

		response := resolveResponse(doc, responseRef)
		if response == nil {
			t.Errorf("%s response %s cannot be resolved", item.label(), status)
			continue
		}
		mediaType := response.Content["application/json"]
		if mediaType == nil {
			t.Errorf("%s response %s must declare a valid application/json content object", item.label(), status)
			continue
		}
		if mediaType.Schema == nil {
			t.Errorf("%s response %s application/json must declare schema", item.label(), status)
			continue
		}
		if !schemaUsesTopLevelComponent(doc, mediaType.Schema, "ApiResponse", map[*openapi3.SchemaRef]bool{}) {
			t.Errorf("%s response %s application/json schema must use ApiResponse as the top-level envelope", item.label(), status)
		}
	}
}

// assertCentralizedErrorResponses 要求非 2xx 响应引用 components.responses。
//
// 错误响应集中维护后，错误码、错误体结构和 requestId 语义只需要在一个地方演进；
// 单个 operation 不应内联一份相似但可能漂移的错误响应定义。
func assertCentralizedErrorResponses(t *testing.T, doc *openapi3.T, item operationItem) {
	t.Helper()

	for status, responseRef := range item.operation.Responses.Map() {
		if is2xxStatus(status) {
			continue
		}
		componentName, ok := strings.CutPrefix(responseRef.Ref, "#/components/responses/")
		if !ok {
			t.Errorf("%s response %s must reference components.responses, got %q", item.label(), status, responseRef.Ref)
			continue
		}
		assertComponentErrorResponse(t, doc, item, status, componentName)
	}
}

// assertComponentErrorResponse 校验某个 operation 非 2xx 响应引用到的组件响应。
//
// operation 级规则只允许 `$ref: '#/components/responses/<Name>'`，但仅检查 `$ref`
// 前缀还不够：组件定义本身也可能漂移成 ApiResponseVoid、内联 object 或缺少 schema。
// 因此这里继续解析 components.responses.<Name>，要求它的 application/json schema
// 顶层使用 ErrorResponse，确保所有业务错误响应共享同一个错误响应模型。
func assertComponentErrorResponse(t *testing.T, doc *openapi3.T, item operationItem, status, componentName string) {
	t.Helper()

	if violation := componentErrorResponseViolation(doc, item, status, componentName); violation != "" {
		t.Errorf("%s", violation)
	}
}

// componentErrorResponseViolation 返回组件错误响应的第一条 policy 违规信息。
//
// 这个函数不直接调用 testing.T，方便单元测试构造“坏的 components.responses”
// 并断言具体违规原因；assertComponentErrorResponse 则负责把违规信息接入主 policy
// 测试的 t.Errorf 聚合输出。空字符串表示组件存在、可解析、声明 application/json，
// 并且 schema 顶层使用 ErrorResponse。
func componentErrorResponseViolation(doc *openapi3.T, item operationItem, status, componentName string) string {
	if doc == nil || doc.Components == nil || doc.Components.Responses == nil {
		return fmt.Sprintf("%s response %s references components.responses.%s but components.responses is missing", item.label(), status, componentName)
	}
	componentRef := doc.Components.Responses[componentName]
	if componentRef == nil {
		return fmt.Sprintf("%s response %s references missing components.responses.%s", item.label(), status, componentName)
	}
	response := resolveResponse(doc, componentRef)
	if response == nil {
		return fmt.Sprintf("%s response %s components.responses.%s cannot be resolved", item.label(), status, componentName)
	}
	mediaType := response.Content["application/json"]
	if mediaType == nil {
		return fmt.Sprintf("%s response %s components.responses.%s must declare a valid application/json content object", item.label(), status, componentName)
	}
	if mediaType.Schema == nil {
		return fmt.Sprintf("%s response %s components.responses.%s application/json must declare schema", item.label(), status, componentName)
	}
	if !schemaUsesTopLevelComponent(doc, mediaType.Schema, "ErrorResponse", map[*openapi3.SchemaRef]bool{}) {
		return fmt.Sprintf("%s response %s components.responses.%s application/json schema must use ErrorResponse as the top-level error schema", item.label(), status, componentName)
	}
	return ""
}

// assertErrorResponseEnvelope 校验 ErrorResponse schema 本身没有脱离统一响应信封。
//
// operation 和 components.responses 都可以正确引用 ErrorResponse，但如果 ErrorResponse
// 自身被改成普通 object，错误响应仍会丢失 code/message/data/requestId/timestamp 这组
// 与 Java ApiResponse 对齐的外层字段。该断言在遍历 operation 前执行，用于提前固定
// OpenAPI 错误模型的全局结构。
func assertErrorResponseEnvelope(t *testing.T, doc *openapi3.T) {
	t.Helper()

	if violation := errorResponseEnvelopeViolation(doc); violation != "" {
		t.Errorf("%s", violation)
	}
}

// errorResponseEnvelopeViolation 返回 ErrorResponse schema 的第一条 envelope 违规信息。
//
// 该 helper 与 schemaUsesTopLevelComponent 共用“只沿 `$ref` 和顶层 allOf 展开”的判断口径：
// 直接引用 ApiResponse 或顶层 allOf 组合 ApiResponse 都合格；仅在 properties 等子树中出现
// ApiResponse 不合格，因为那不能证明错误响应最外层就是统一 envelope。空字符串表示合格。
func errorResponseEnvelopeViolation(doc *openapi3.T) string {
	if doc == nil || doc.Components == nil || doc.Components.Schemas == nil {
		return "components.schemas must declare ErrorResponse"
	}
	errorResponseRef := doc.Components.Schemas["ErrorResponse"]
	if errorResponseRef == nil {
		return "components.schemas.ErrorResponse must exist"
	}
	if !schemaUsesTopLevelComponent(doc, errorResponseRef, "ApiResponse", map[*openapi3.SchemaRef]bool{}) {
		return "components.schemas.ErrorResponse must use ApiResponse as the top-level envelope"
	}
	return ""
}

// assertErrorResponseViolations 固化字段校验错误的 data.violations wire contract。
func assertErrorResponseViolations(t *testing.T, doc *openapi3.T) {
	t.Helper()

	if violation := errorResponseViolationsViolation(doc); violation != "" {
		t.Errorf("%s", violation)
	}
}

// errorResponseViolationsViolation 返回 ErrorResponse.data.violations 的第一条结构违规。
//
// violations 对全局 ErrorResponse.data 保持可选，因为非字段类错误仍允许 data=null 或
// 携带其它 AppError details；一旦出现 violations，每一项必须固定为五字段 Violation。
func errorResponseViolationsViolation(doc *openapi3.T) string {
	if doc == nil || doc.Components == nil || doc.Components.Schemas == nil {
		return "components.schemas must declare ErrorResponse and Violation"
	}
	errorResponseRef := doc.Components.Schemas["ErrorResponse"]
	if errorResponseRef == nil || errorResponseRef.Value == nil {
		return "components.schemas.ErrorResponse must exist"
	}

	candidates := append(openapi3.SchemaRefs{errorResponseRef}, errorResponseRef.Value.AllOf...)
	var dataSchema *openapi3.Schema
	var violationsRef *openapi3.SchemaRef
	for _, candidate := range candidates {
		if candidate == nil || candidate.Value == nil {
			continue
		}
		dataRef := candidate.Value.Properties["data"]
		if dataRef == nil || dataRef.Value == nil {
			continue
		}
		candidateViolations := dataRef.Value.Properties["violations"]
		if candidateViolations == nil {
			continue
		}
		dataSchema = dataRef.Value
		violationsRef = candidateViolations
		break
	}
	if dataSchema == nil || violationsRef == nil || violationsRef.Value == nil {
		return "components.schemas.ErrorResponse.data must declare optional violations"
	}
	if contains(dataSchema.Required, "violations") {
		return "components.schemas.ErrorResponse.data.violations must remain optional"
	}
	if !dataSchema.Nullable {
		return "components.schemas.ErrorResponse.data must remain nullable"
	}
	if dataSchema.AdditionalProperties.Has == nil || !*dataSchema.AdditionalProperties.Has {
		return "components.schemas.ErrorResponse.data must preserve additionalProperties for non-validation details"
	}
	if !violationsRef.Value.Type.Is("array") || violationsRef.Value.Items == nil {
		return "components.schemas.ErrorResponse.data.violations must be an array of Violation"
	}
	if !schemaUsesTopLevelComponent(doc, violationsRef.Value.Items, "Violation", map[*openapi3.SchemaRef]bool{}) {
		return "components.schemas.ErrorResponse.data.violations items must use Violation"
	}

	violationRef := doc.Components.Schemas["Violation"]
	if violationRef == nil || violationRef.Value == nil || !violationRef.Value.Type.Is("object") {
		return "components.schemas.Violation must be an object"
	}
	if violationRef.Value.AdditionalProperties.Has == nil || *violationRef.Value.AdditionalProperties.Has {
		return "components.schemas.Violation must forbid additionalProperties"
	}
	wantFields := []string{"location", "field", "path", "rule", "message"}
	if len(violationRef.Value.Required) != len(wantFields) {
		return "components.schemas.Violation must require location, field, path, rule, message"
	}
	for _, field := range wantFields {
		if !contains(violationRef.Value.Required, field) {
			return fmt.Sprintf("components.schemas.Violation must require %s", field)
		}
		fieldRef := violationRef.Value.Properties[field]
		if fieldRef == nil || fieldRef.Value == nil || !fieldRef.Value.Type.Is("string") {
			return fmt.Sprintf("components.schemas.Violation.%s must be a string", field)
		}
	}
	return ""
}

// assertAdminSecurityPolicy 固化管理员接口的机器可验证授权声明。
//
// BearerAuth 表示接口需要已认证主体；x-required-roles 是团队自定义 OpenAPI 扩展，
// 用于表达“该接口必须具备 ADMIN 角色”。description 中的文字说明可以保留给人读，
// 但测试只信任结构化字段，避免后续文档描述和真实安全边界出现漂移。
func assertAdminSecurityPolicy(t *testing.T, item operationItem) {
	t.Helper()

	if violation := adminSecurityPolicyViolation(item); violation != "" {
		t.Errorf("%s", violation)
	}
}

// adminSecurityPolicyViolation 返回管理员接口安全声明的第一条 policy 违规信息。
//
// 管理员接口必须同时声明 BearerAuth 和精确的 `x-required-roles: [ADMIN]`。
// 这里刻意不接受额外角色，避免 OpenAPI 文档把真实运行时的 ADMIN-only 边界写宽。
func adminSecurityPolicyViolation(item operationItem) string {
	if !hasBearerAuth(item.operation) {
		return fmt.Sprintf("%s must declare BearerAuth security", item.label())
	}

	roles, ok := requiredRoles(item.operation)
	if !ok {
		return fmt.Sprintf("%s must declare x-required-roles as a string array", item.label())
	}
	if len(roles) != 1 || roles[0] != "ADMIN" {
		return fmt.Sprintf("%s x-required-roles must equal [ADMIN], got %v", item.label(), roles)
	}
	return ""
}

// assertNonAdminRolePolicy 防止普通业务接口误声明 ADMIN 角色。
func assertNonAdminRolePolicy(t *testing.T, item operationItem) {
	t.Helper()

	if violation := adminRoleLeakViolation(item); violation != "" {
		t.Errorf("%s", violation)
	}
}

// adminRoleLeakViolation 返回非管理员接口误声明 ADMIN 的 policy 违规信息。
func adminRoleLeakViolation(item operationItem) string {
	roles, ok := requiredRoles(item.operation)
	if !ok {
		return ""
	}
	if contains(roles, "ADMIN") {
		return fmt.Sprintf("%s non-admin operation must not declare ADMIN in x-required-roles, got %v", item.label(), roles)
	}
	return ""
}

// assertAuthSecurityPolicy 单独固定认证相关公开/受保护接口的安全策略。
//
// 不能为了让管理员接口通过而盲目给所有接口加 BearerAuth：register、login、refresh
// 是当前业务允许匿名调用的入口；logout 和 /api/v1/me 则必须依赖当前认证主体。
func assertAuthSecurityPolicy(t *testing.T, operations []operationItem) {
	t.Helper()

	expectedBearerAuth := map[string]bool{
		"POST /api/v1/auth/register": false,
		"POST /api/v1/auth/login":    false,
		"POST /api/v1/auth/refresh":  false,
		"POST /api/v1/auth/logout":   true,
		"GET /api/v1/me":             true,
	}

	byRoute := make(map[string]operationItem, len(operations))
	for _, item := range operations {
		byRoute[item.routeKey()] = item
	}

	for route, wantBearerAuth := range expectedBearerAuth {
		item, ok := byRoute[route]
		if !ok {
			t.Errorf("%s must exist in OpenAPI spec", route)
			continue
		}
		gotBearerAuth := hasBearerAuth(item.operation)
		if gotBearerAuth != wantBearerAuth {
			t.Errorf("%s expected BearerAuth=%v, got %v", item.label(), wantBearerAuth, gotBearerAuth)
		}
	}
}

// assertBearerAuthSecurityScheme 固化运行时 security bridge 依赖的认证方案。
//
// BearerAuth 不只是文档展示字段；contract gate 会根据 operation security 中的
// BearerAuth 触发现有认证能力。因此 components.securitySchemes 中必须存在同名
// http bearer scheme，避免 operation 级策略看似存在但运行时无法正确解释。
func assertBearerAuthSecurityScheme(t *testing.T, doc *openapi3.T) {
	t.Helper()

	if doc == nil || doc.Components == nil || doc.Components.SecuritySchemes == nil {
		t.Errorf("components.securitySchemes must declare BearerAuth")
		return
	}
	schemeRef := doc.Components.SecuritySchemes["BearerAuth"]
	if schemeRef == nil || schemeRef.Value == nil {
		t.Errorf("components.securitySchemes.BearerAuth must exist")
		return
	}
	scheme := schemeRef.Value
	if scheme.Type != "http" {
		t.Errorf("components.securitySchemes.BearerAuth type: got %q want http", scheme.Type)
	}
	if scheme.Scheme != "bearer" {
		t.Errorf("components.securitySchemes.BearerAuth scheme: got %q want bearer", scheme.Scheme)
	}
}

// resolveResponse 把 OpenAPI response 引用解析成实际 Response 对象。
//
// kin-openapi 的 ResponseRef 同时支持两种形态：Value 表示当前 operation 里内联定义；
// Ref 表示通过 $ref 指向 components.responses。策略测试需要读取 content/schema，
// 因此先把这两种形态统一解析成 *openapi3.Response。
func resolveResponse(doc *openapi3.T, responseRef *openapi3.ResponseRef) *openapi3.Response {
	if responseRef == nil {
		return nil
	}
	if responseRef.Value != nil {
		return responseRef.Value
	}
	const prefix = "#/components/responses/"
	name, ok := strings.CutPrefix(responseRef.Ref, prefix)
	if !ok {
		return nil
	}
	componentRef := doc.Components.Responses[name]
	if componentRef == nil {
		return nil
	}
	return componentRef.Value
}

// schemaUsesTopLevelComponent 判断一个 schema 是否以指定 components.schemas 作为顶层结构。
//
// SchemaRef 是 OpenAPI schema 的包装层：Ref 表示 $ref，Value 表示内联 schema。
// ApiResponse envelope 的合法形态包括：
//   - 直接 $ref: '#/components/schemas/ApiResponse'
//   - 先引用某个响应 schema，再由该 schema allOf 组合 ApiResponse
//
// 函数只沿 $ref 和顶层 allOf 展开，不扫描 properties、items、oneOf 或 anyOf。
// 这样可以证明响应最外层是 ApiResponse，而不是仅在任意子 schema 中出现过 ApiResponse。
// visited 用于避免循环引用导致无限递归。
func schemaUsesTopLevelComponent(doc *openapi3.T, schemaRef *openapi3.SchemaRef, target string, visited map[*openapi3.SchemaRef]bool) bool {
	if schemaRef == nil {
		return false
	}
	// 最简单、最理想的情况：当前 schema 自身就是目标组件引用。
	if schemaRef.Ref == "#/components/schemas/"+target {
		return true
	}
	if visited[schemaRef] {
		return false
	}
	visited[schemaRef] = true

	// 如果当前 schemaRef 指向另一个 components.schemas，继续检查被引用组件的实际定义。
	// 这覆盖了 ListAdminUsersResponse 这类“命名响应 schema 再 allOf ApiResponse”的写法。
	if name, ok := strings.CutPrefix(schemaRef.Ref, "#/components/schemas/"); ok {
		if doc == nil || doc.Components == nil || doc.Components.Schemas == nil {
			return false
		}
		componentRef := doc.Components.Schemas[name]
		if componentRef != nil && schemaUsesTopLevelComponent(doc, componentRef, target, visited) {
			return true
		}
	}

	schema := schemaRef.Value
	if schema == nil {
		return false
	}

	// allOf 是当前 OpenAPI 中表达“继承 ApiResponse，并约束 data 具体类型”的顶层组合方式。
	for _, child := range schema.AllOf {
		if schemaUsesTopLevelComponent(doc, child, target, visited) {
			return true
		}
	}

	return false
}

// hasBearerAuth 判断 operation 是否显式声明 BearerAuth。
//
// OpenAPI security 是“多个 requirement 之间 OR、单个 requirement 内多个 scheme AND”的结构。
// 这里的 policy 只关心是否存在任意一个 requirement 包含 BearerAuth。
func hasBearerAuth(operation *openapi3.Operation) bool {
	if operation.Security == nil {
		return false
	}
	for _, requirement := range *operation.Security {
		if _, ok := requirement["BearerAuth"]; ok {
			return true
		}
	}
	return false
}

// requiredRoles 读取团队自定义扩展 x-required-roles。
//
// YAML 解析后扩展字段通常是 []any；测试也兼容 []string，方便未来如果加载器行为变化，
// 或者有测试直接构造 openapi3.Operation，仍能复用同一个解析逻辑。
func requiredRoles(operation *openapi3.Operation) ([]string, bool) {
	rawRoles, ok := operation.Extensions["x-required-roles"]
	if !ok {
		return nil, false
	}

	switch roles := rawRoles.(type) {
	case []string:
		return roles, true
	case []any:
		values := make([]string, 0, len(roles))
		for _, role := range roles {
			value, ok := role.(string)
			if !ok {
				return nil, false
			}
			values = append(values, value)
		}
		return values, true
	default:
		return nil, false
	}
}

// is2xxStatus 只识别明确的三位 2xx 状态码；default、4xx、5xx 都不属于成功响应。
func is2xxStatus(status string) bool {
	return len(status) == 3 && status[0] == '2'
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
