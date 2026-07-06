package system

import (
	"strings"
	"unicode/utf8"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/validation"
	systemsvc "eventhub-go/internal/service/system"
)

// parseEchoCommand 校验 system echo 请求并映射为 service command。
func parseEchoCommand(request *openapigen.EchoRequest) (systemsvc.EchoCommand, *apperror.AppError) {
	if request == nil {
		return systemsvc.EchoCommand{}, validation.MalformedBodyError()
	}

	fields := validation.FieldErrors{}

	if strings.TrimSpace(request.Message) == "" {
		fields["message"] = "message 不能为空"
	} else if utf8.RuneCountInString(request.Message) > 64 {
		fields["message"] = "message 长度不能超过 64"
	}

	if request.Tag != nil && utf8.RuneCountInString(*request.Tag) > 32 {
		fields["tag"] = "tag 长度不能超过 32"
	}

	if len(fields) > 0 {
		return systemsvc.EchoCommand{}, validation.BodyValidationError(fields)
	}
	return systemsvc.EchoCommand{
		Message: request.Message,
		Tag:     request.Tag,
	}, nil
}
