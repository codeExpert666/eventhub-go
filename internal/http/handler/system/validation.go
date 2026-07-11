package system

import (
	"strings"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
	systemsvc "eventhub-go/internal/service/system"
)

// parseEchoCommand 校验 system echo 请求并映射为 service command。
func parseEchoCommand(request *openapigen.EchoRequest) (systemsvc.EchoCommand, *apperror.AppError) {
	if request == nil {
		return systemsvc.EchoCommand{}, requesterror.MissingBody()
	}

	violations := requesterror.Violations{}

	if strings.TrimSpace(request.Message) == "" {
		violations = append(violations, requesterror.Violation{
			Location: requesterror.LocationBody,
			Field:    "message",
			Path:     "message",
			Rule:     "notBlank",
			Message:  "message 不能为空",
		})
	}

	if len(violations) > 0 {
		return systemsvc.EchoCommand{}, requesterror.InvalidBody(violations)
	}
	return systemsvc.EchoCommand{
		Message: request.Message,
		Tag:     request.Tag,
	}, nil
}
