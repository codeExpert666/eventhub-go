package apperror

// AppError 表示可被统一映射为 API 响应的应用错误。
type AppError struct {
	code    Code
	message string
	data    any
	cause   error
}

// New 根据错误码和可选自定义消息创建 AppError。
// message 为空时使用错误码的默认用户提示。
func New(code Code, message string) *AppError {
	code = normalizeCode(code)
	if message == "" {
		message = code.DefaultMessage()
	}
	return &AppError{code: code, message: message}
}

// WithData 创建携带结构化错误数据的 AppError。
func WithData(code Code, message string, data any) *AppError {
	err := New(code, message)
	err.data = data
	return err
}

// Wrap 创建包装底层错误原因的 AppError。
func Wrap(code Code, message string, cause error) *AppError {
	err := New(code, message)
	err.cause = cause
	return err
}

// Error 返回用户可读的错误消息。
func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

// Unwrap 返回被包装的底层错误原因。
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Code 返回稳定的应用错误码，空接收者退化为 CommonInternal。
func (e *AppError) Code() Code {
	if e == nil {
		return CommonInternal
	}
	return e.code
}

// Message 返回用户可读的错误消息，空接收者退化为通用内部错误提示。
func (e *AppError) Message() string {
	if e == nil {
		return CommonInternal.DefaultMessage()
	}
	return e.message
}

// Data 返回可选的结构化错误数据。
func (e *AppError) Data() any {
	if e == nil {
		return nil
	}
	return e.data
}
