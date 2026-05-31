package handler

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/config"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/http/validation"
)

type SystemHandler struct {
	cfg config.Config
}

func NewSystemHandler(cfg config.Config) *SystemHandler {
	return &SystemHandler{cfg: cfg}
}

type PingInfo struct {
	ServiceName    string    `json:"serviceName"`
	ActiveProfiles []string  `json:"activeProfiles"`
	ServerTime     time.Time `json:"serverTime"`
}

type EchoRequest struct {
	Message string  `json:"message"`
	Tag     *string `json:"tag"`
}

type EchoInfo struct {
	Message  string    `json:"message"`
	Tag      *string   `json:"tag"`
	EchoedAt time.Time `json:"echoedAt"`
}

type HealthInfo struct {
	Status string `json:"status"`
}

type Info struct {
	App     AppInfo `json:"app"`
	Runtime Runtime `json:"runtime"`
}

type AppInfo struct {
	Name           string   `json:"name"`
	Env            string   `json:"env"`
	Version        string   `json:"version"`
	ActiveProfiles []string `json:"activeProfiles"`
}

type Runtime struct {
	ServerTime time.Time `json:"serverTime"`
}

func (h *SystemHandler) Ping(w http.ResponseWriter, r *http.Request) {
	response.WriteSuccess(w, r, PingInfo{
		ServiceName:    h.cfg.AppName,
		ActiveProfiles: h.cfg.ActiveProfiles(),
		ServerTime:     time.Now(),
	})
}

func (h *SystemHandler) Echo(w http.ResponseWriter, r *http.Request) {
	var request EchoRequest
	if err := validation.DecodeJSONBody(r, &request); err != nil {
		response.WriteError(w, r, err)
		return
	}

	if err := validateEchoRequest(request); err != nil {
		response.WriteError(w, r, err)
		return
	}

	response.WriteSuccess(w, r, EchoInfo{
		Message:  request.Message,
		Tag:      request.Tag,
		EchoedAt: time.Now(),
	})
}

func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	response.WriteJSON(w, http.StatusOK, HealthInfo{Status: "UP"})
}

func (h *SystemHandler) HealthHead(w http.ResponseWriter, r *http.Request) {
	response.WriteStatus(w, http.StatusOK)
}

func (h *SystemHandler) Info(w http.ResponseWriter, r *http.Request) {
	response.WriteJSON(w, http.StatusOK, Info{
		App: AppInfo{
			Name:           h.cfg.AppName,
			Env:            h.cfg.Env,
			Version:        h.cfg.Version,
			ActiveProfiles: h.cfg.ActiveProfiles(),
		},
		Runtime: Runtime{ServerTime: time.Now()},
	})
}

func (h *SystemHandler) InfoHead(w http.ResponseWriter, r *http.Request) {
	response.WriteStatus(w, http.StatusOK)
}

func validateEchoRequest(request EchoRequest) *apperror.AppError {
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
		return validation.BodyValidationError(fields)
	}
	return nil
}
