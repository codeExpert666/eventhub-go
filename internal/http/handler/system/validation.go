package system

import (
	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
	systemsvc "eventhub-go/internal/service/system"
)

// parseEchoCommand 防御 nil 请求体，并将 generated request 原样映射为 service command。
func parseEchoCommand(request *openapigen.EchoRequest) (systemsvc.EchoCommand, *apperror.AppError) {
	if request == nil {
		return systemsvc.EchoCommand{}, requesterror.MalformedBody()
	}

	return systemsvc.EchoCommand{
		Message: request.Message,
		Tag:     request.Tag,
	}, nil
}
