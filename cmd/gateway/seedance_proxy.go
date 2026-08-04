package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	seedanceusage "github.com/HyperToken-dev/fabric/business/usage/seedance"
	coreseedance "github.com/HyperToken-dev/fabric/core/providers/seedance"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	"github.com/HyperToken-dev/fabric/internal/models"
	"go.uber.org/zap"
)

const (
	seedanceTasksPath = "/api/v3/contents/generations/tasks"
)

type SeedanceProxy struct {
	coreProxy    *coreproxy.Proxy
	modelStore   ModelStore
	integralLogs IntegralLogHandler
	tasks        ProviderTaskStore
	textPolicy   TextPolicy
}

type SeedanceProxyOptions struct {
	ModelStore         ModelStore
	IntegralLogHandler IntegralLogHandler
	ProviderTaskStore  ProviderTaskStore
	TextPolicy         TextPolicy
}

type seedanceVideoTaskRequest struct {
	Model   string                 `json:"model"`
	Content []seedanceContentEntry `json:"content"`
}

type seedanceContentEntry struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type seedanceTaskResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Status string `json:"status"`
}

func NewSeedanceProxy(opts SeedanceProxyOptions) (*SeedanceProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = NoopModelStore{}
	}
	if opts.IntegralLogHandler == nil {
		opts.IntegralLogHandler = NoopIntegralLogHandler{}
	}
	if opts.ProviderTaskStore == nil {
		opts.ProviderTaskStore = NoopProviderTaskStore{}
	}
	if opts.TextPolicy == nil {
		opts.TextPolicy = NoopTextPolicy{}
	}
	p := &SeedanceProxy{modelStore: opts.ModelStore, integralLogs: opts.IntegralLogHandler, tasks: opts.ProviderTaskStore, textPolicy: opts.TextPolicy}
	p.coreProxy = coreseedance.New(coreproxy.Options{OnComplete: p.onComplete})
	return p, nil
}

func (p *SeedanceProxy) onComplete(resp *http.Response, decodedBody []byte) {
	keyID := getContextInt32(resp.Request, ctxKeyID)
	channelID := getContextInt32(resp.Request, ctxChannelID)
	model := getContextString(resp.Request, ctxModel)
	modelID := getContextInt32(resp.Request, ctxModelID)
	taskResp, taskRespOK := parseSeedanceTaskResponse(decodedBody)
	if model == "" && taskRespOK {
		model = taskResp.Model
	}
	info := integralLogInfo{
		Provider:                ProviderSeedance,
		APIFormat:               models.APIFormatSeedance,
		KeyID:                   keyID,
		ChannelID:               channelID,
		ModelID:                 modelID,
		Model:                   model,
		Outcome:                 responseOutcome(resp.StatusCode),
		ResponseStatus:          resp.StatusCode,
		ResponseContentType:     resp.Header.Get("Content-Type"),
		ResponseContentEncoding: resp.Header.Get("Content-Encoding"),
		DecodeOK:                decodedBody != nil,
	}
	if decodedBody != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices && taskRespOK {
		go p.processTaskResponseAsync(resp.Request, taskResp, decodedBody, keyID, channelID, modelID)
	}
	go processIntegralLogAsync(p.integralLogs, resp.Request, info, decodedBody)
}

func (p *SeedanceProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	if strings.TrimSpace(providerKey) == "" {
		zap.L().Error("missing provider key", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("provider", string(ProviderSeedance)), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing provider key", http.StatusBadGateway)
		return
	}

	requestKind := classifySeedanceRequest(r, baseURL)
	switch requestKind {
	case seedanceRequestCreateTask:
		modelID, modelName, requestBody, err := p.prepareCreateTaskRequest(r, channelID)
		if err != nil {
			if errors.Is(err, errPromptRejected) {
				p.writePromptRejected(w, r, keyID, channelID, modelID, modelName)
				return
			}
			p.writeModelError(w, r, keyID, channelID, err)
			return
		}
		zap.L().Info("seedance video task request received", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
		r = setContextString(r, ctxModel, modelName)
		r = setContextInt32(r, ctxModelID, modelID)
		r = setContextRawJSON(r, ctxSeedanceCreateRequest, requestBody)
	case seedanceRequestUnsupported:
		zap.L().Warn("unsupported seedance proxy request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID))
		http.Error(w, "unsupported seedance path", http.StatusNotFound)
		return
	}

	r = setContextInt32(r, ctxKeyID, keyID)
	r = setContextInt32(r, ctxChannelID, channelID)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}

func (p *SeedanceProxy) prepareCreateTaskRequest(r *http.Request, channelID int32) (int32, string, json.RawMessage, error) {
	if r.Body == nil {
		return 0, "", nil, errMissingModel
	}
	body, err := io.ReadAll(r.Body)
	closeErr := r.Body.Close()
	if err != nil {
		if closeErr != nil {
			return 0, "", nil, errors.Join(err, closeErr)
		}
		return 0, "", nil, err
	}
	restoreRequestBody(r, body)
	if closeErr != nil {
		return 0, "", nil, closeErr
	}

	var payload seedanceVideoTaskRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, "", nil, errInvalidRequestBody
	}
	modelName := strings.TrimSpace(payload.Model)
	if modelName == "" {
		return 0, "", nil, errMissingModel
	}
	modelID, err := resolveModelFromStore(r.Context(), p.modelStore, channelID, modelName)
	if err != nil {
		return 0, "", nil, err
	}
	if detectPrompts(r.Context(), modelName, TextDirectionInput, seedancePromptTexts(payload), p.textPolicy) {
		return modelID, modelName, append(json.RawMessage(nil), body...), errPromptRejected
	}
	return modelID, modelName, append(json.RawMessage(nil), body...), nil
}

func (p *SeedanceProxy) writeModelError(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, err error) {
	switch {
	case errors.Is(err, errInvalidRequestBody):
		zap.L().Error("parse seedance request failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "invalid request body", http.StatusBadRequest)
	case errors.Is(err, errMissingModel):
		zap.L().Warn("missing seedance model", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing model", http.StatusBadRequest)
	case errors.Is(err, errModelDisabled):
		zap.L().Warn("seedance model disabled", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "model disabled", http.StatusForbidden)
	case errors.Is(err, errModelUnsupported):
		zap.L().Warn("seedance model unsupported", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "unsupported model", http.StatusBadRequest)
	default:
		zap.L().Error("resolve seedance model failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "model lookup failed", http.StatusInternalServerError)
	}
}

func (p *SeedanceProxy) writePromptRejected(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, modelID int32, modelName string) {
	zap.L().Warn("seedance prompt rejected", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	responseBody := []byte("prompt rejected\n")
	http.Error(w, "prompt rejected", http.StatusForbidden)
	go processIntegralLogAsync(p.integralLogs, r, integralLogInfo{
		Provider:            ProviderSeedance,
		APIFormat:           models.APIFormatSeedance,
		KeyID:               keyID,
		ChannelID:           channelID,
		ModelID:             modelID,
		Model:               modelName,
		Outcome:             integralOutcomeRejected,
		RejectionStage:      rejectionStageInput,
		RejectionReason:     rejectionReasonSensitive,
		ResponseStatus:      http.StatusForbidden,
		ResponseContentType: "text/plain; charset=utf-8",
		DecodeOK:            true,
	}, responseBody)
}

type seedanceRequestKind int

const (
	seedanceRequestUnsupported seedanceRequestKind = iota
	seedanceRequestCreateTask
	seedanceRequestManageTask
)

const ctxSeedanceCreateRequest contextKey = "seedance_create_request"

func classifySeedanceRequest(r *http.Request, baseURL string) seedanceRequestKind {
	effectivePath := seedanceEffectivePath(baseURL, r.URL.Path)
	if effectivePath == seedanceTasksPath {
		if r.Method == http.MethodPost {
			return seedanceRequestCreateTask
		}
		if r.Method == http.MethodGet {
			return seedanceRequestManageTask
		}
		return seedanceRequestUnsupported
	}
	if strings.HasPrefix(effectivePath, seedanceTasksPath+"/") && strings.TrimPrefix(effectivePath, seedanceTasksPath+"/") != "" {
		if r.Method == http.MethodGet || r.Method == http.MethodDelete {
			return seedanceRequestManageTask
		}
	}
	return seedanceRequestUnsupported
}

func seedanceEffectivePath(baseURL string, requestPath string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Path == "" || parsed.Path == "/" {
		return path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	}
	if requestPath == "" || requestPath == "/" {
		return path.Clean(parsed.Path)
	}
	return path.Clean(parsed.Path + "/" + strings.TrimPrefix(requestPath, "/"))
}

func seedancePromptTexts(payload seedanceVideoTaskRequest) []string {
	prompts := make([]string, 0, len(payload.Content))
	for _, entry := range payload.Content {
		if strings.EqualFold(strings.TrimSpace(entry.Type), "text") && strings.TrimSpace(entry.Text) != "" {
			prompts = append(prompts, entry.Text)
		}
	}
	return prompts
}

func parseSeedanceTaskResponse(body []byte) (seedanceTaskResponse, bool) {
	if len(body) == 0 {
		return seedanceTaskResponse{}, false
	}
	var resp seedanceTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return seedanceTaskResponse{}, false
	}
	resp.ID = strings.TrimSpace(resp.ID)
	resp.Model = strings.TrimSpace(resp.Model)
	return resp, resp.ID != ""
}

func (p *SeedanceProxy) processTaskResponseAsync(req *http.Request, taskResp seedanceTaskResponse, rawBody []byte, keyID int32, channelID int32, modelID int32) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status := normalizeProviderTaskStatus(taskResp.Status)
	requestBody := getContextRawJSON(req, ctxSeedanceCreateRequest)
	if len(requestBody) > 0 {
		if modelID == 0 {
			zap.L().Error("missing resolved model id for seedance provider task", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("provider_task_id", taskResp.ID), zap.String("model", taskResp.Model))
			return
		}
		if err := p.tasks.CreateProviderTask(ctx, ProviderTaskInfo{
			Provider:       ProviderSeedance,
			KeyID:          keyID,
			ChannelID:      channelID,
			ModelID:        modelID,
			ProviderTaskID: taskResp.ID,
			Status:         status,
			Request:        requestBody,
			Response:       append(json.RawMessage(nil), rawBody...),
		}); err != nil {
			zap.L().Error("create seedance provider task failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("provider_task_id", taskResp.ID))
			return
		}
	}

	if !isTerminalProviderTaskStatus(status) {
		_, err := p.tasks.CompleteProviderTask(ctx, ProviderTaskCompletion{
			Provider:       ProviderSeedance,
			ProviderTaskID: taskResp.ID,
			Status:         status,
			Response:       append(json.RawMessage(nil), rawBody...),
		})
		if err != nil {
			zap.L().Error("update seedance provider task failed", zap.Error(err), zap.String("provider_task_id", taskResp.ID))
		}
		return
	}

	completionTokens := int64(0)
	parsedUsage, usageErr := seedanceusage.ExtractTaskUsage(rawBody)
	if usageErr != nil {
		zap.L().Error("extract seedance task usage failed", zap.Error(usageErr), zap.String("provider_task_id", taskResp.ID))
	} else if parsedUsage != nil {
		completionTokens = parsedUsage.CompletionTokens
	}

	inserted, err := p.tasks.CompleteProviderTask(ctx, ProviderTaskCompletion{
		Provider:         ProviderSeedance,
		ProviderTaskID:   taskResp.ID,
		Status:           status,
		Response:         append(json.RawMessage(nil), rawBody...),
		CompletionTokens: completionTokens,
	})
	if err != nil {
		zap.L().Error("complete seedance provider task failed", zap.Error(err), zap.String("provider_task_id", taskResp.ID), zap.Int64("completion_tokens", completionTokens))
		return
	}
	if inserted {
		zap.L().Info("seedance provider task usage recorded", zap.String("provider_task_id", taskResp.ID), zap.Int64("completion_tokens", completionTokens))
	}
}

func setContextRawJSON(r *http.Request, key contextKey, val json.RawMessage) *http.Request {
	ctx := context.WithValue(r.Context(), key, append(json.RawMessage(nil), val...))
	return r.WithContext(ctx)
}

func getContextRawJSON(r *http.Request, key contextKey) json.RawMessage {
	v, _ := r.Context().Value(key).(json.RawMessage)
	return v
}

var errPromptRejected = errors.New("prompt rejected")
