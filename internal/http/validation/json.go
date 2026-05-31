package validation

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"eventhub-go/internal/apperror"
)

// FieldErrors 表示按字段名归集的请求参数校验错误。
type FieldErrors map[string]string

// DecodeJSONBody 将 HTTP 请求体中的 JSON 解码到 dst，并统一映射请求体格式错误。
// 该函数要求请求体存在且只包含一个 JSON 值，避免客户端提交尾随内容被静默忽略。
func DecodeJSONBody(r *http.Request, dst any) *apperror.AppError {
	if r.Body == nil {
		return malformedBodyError()
	}

	decoder := json.NewDecoder(r.Body)
	// Decode 会按 dst 结构体字段的 json tag 映射请求字段；未知字段默认忽略。
	if err := decoder.Decode(dst); err != nil {
		return malformedBodyError()
	}

	// 第二次 Decode 只用于发现首个 JSON 值后的非空白尾随内容；extra 不参与业务字段映射。
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return malformedBodyError()
	}
	return nil
}

// BodyValidationError 根据字段校验结果构造统一的请求体参数校验错误。
func BodyValidationError(fields FieldErrors) *apperror.AppError {
	return apperror.WithData(
		apperror.CommonValidation,
		"请求体参数校验失败",
		fields,
	)
}

// malformedBodyError 构造请求体缺失、格式错误或包含非法内容时的统一错误。
func malformedBodyError() *apperror.AppError {
	return apperror.WithData(
		apperror.CommonValidation,
		"请求体格式不合法",
		map[string]string{"body": "请求体缺失或 JSON 格式错误"},
	)
}
