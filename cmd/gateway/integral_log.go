package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	integralOutcomeOK       = "ok"
	integralOutcomeError    = "error"
	integralOutcomeRejected = "rejected"

	rejectionStageInput      = "input"
	rejectionStageOutput     = "output"
	rejectionReasonSensitive = "sensitive"
)

type integralLogInfo struct {
	Provider                Provider
	APIFormat               int32
	KeyID                   int32
	ChannelID               int32
	ModelID                 int32
	Model                   string
	Outcome                 string
	RejectionStage          string
	RejectionReason         string
	ResponseStatus          int
	ResponseContentType     string
	ResponseContentEncoding string
	DecodeOK                bool
}

func processIntegralLogAsync(handler IntegralLogHandler, req *http.Request, info integralLogInfo, responseBody []byte) {
	if handler == nil {
		handler = NoopIntegralLogHandler{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contextJSON := integralLogContext(req, info)
	if err := handler.InsertIntegralLog(ctx, info.KeyID, contextJSON, string(responseBody)); err != nil {
		zap.L().Error("insert integral log failed", zap.Error(err), zap.Int32("key_id", info.KeyID), zap.Int32("channel_id", info.ChannelID), zap.Int32("model_id", info.ModelID), zap.String("model", info.Model), zap.String("provider", string(info.Provider)))
		return
	}
	zap.L().Info("integral log inserted", zap.Int32("key_id", info.KeyID), zap.Int32("channel_id", info.ChannelID), zap.Int32("model_id", info.ModelID), zap.String("model", info.Model), zap.String("provider", string(info.Provider)), zap.String("outcome", info.Outcome), zap.Int("response_bytes", len(responseBody)))
}

func integralLogContext(req *http.Request, info integralLogInfo) string {
	if info.Outcome == "" {
		info.Outcome = integralOutcomeOK
	}
	entry := struct {
		Provider                string          `json:"provider"`
		APIFormat               int32           `json:"api_format"`
		Outcome                 string          `json:"outcome"`
		RejectionStage          string          `json:"rejection_stage,omitempty"`
		RejectionReason         string          `json:"rejection_reason,omitempty"`
		Method                  string          `json:"method"`
		Path                    string          `json:"path"`
		Query                   string          `json:"query,omitempty"`
		Model                   string          `json:"model"`
		ModelID                 int32           `json:"model_id"`
		ChannelID               int32           `json:"channel_id"`
		ResponseStatus          int             `json:"response_status,omitempty"`
		ResponseContentType     string          `json:"response_content_type,omitempty"`
		ResponseContentEncoding string          `json:"response_content_encoding,omitempty"`
		DecodeOK                bool            `json:"decode_ok"`
		Request                 json.RawMessage `json:"request"`
	}{
		Provider:                string(info.Provider),
		APIFormat:               info.APIFormat,
		Outcome:                 info.Outcome,
		RejectionStage:          info.RejectionStage,
		RejectionReason:         info.RejectionReason,
		Model:                   info.Model,
		ModelID:                 info.ModelID,
		ChannelID:               info.ChannelID,
		ResponseStatus:          info.ResponseStatus,
		ResponseContentType:     info.ResponseContentType,
		ResponseContentEncoding: info.ResponseContentEncoding,
		DecodeOK:                info.DecodeOK,
		Request:                 json.RawMessage(`null`),
	}
	if req != nil {
		entry.Method = req.Method
		if req.URL != nil {
			entry.Path = req.URL.Path
			entry.Query = req.URL.RawQuery
		}
	}

	if req != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			zap.L().Error("get request body for integral log failed", zap.Error(err), zap.Int32("key_id", info.KeyID), zap.Int32("channel_id", info.ChannelID), zap.Int32("model_id", info.ModelID), zap.String("model", info.Model))
		} else {
			raw, readErr := io.ReadAll(body)
			closeErr := body.Close()
			if readErr != nil {
				zap.L().Error("read request body for integral log failed", zap.Error(readErr), zap.Int32("key_id", info.KeyID), zap.Int32("channel_id", info.ChannelID), zap.Int32("model_id", info.ModelID), zap.String("model", info.Model))
			} else if len(strings.TrimSpace(string(raw))) > 0 && json.Valid(raw) {
				entry.Request = append(json.RawMessage(nil), raw...)
			} else if len(strings.TrimSpace(string(raw))) > 0 {
				zap.L().Error("request body for integral log is not valid JSON", zap.String("raw_body_prefix", bodyPrefix(raw, 128)), zap.Int32("key_id", info.KeyID), zap.Int32("channel_id", info.ChannelID), zap.Int32("model_id", info.ModelID), zap.String("model", info.Model))
			}
			if closeErr != nil {
				zap.L().Error("close request body for integral log failed", zap.Error(closeErr), zap.Int32("key_id", info.KeyID), zap.Int32("channel_id", info.ChannelID), zap.Int32("model_id", info.ModelID), zap.String("model", info.Model))
			}
		}
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		zap.L().Error("marshal integral log context failed", zap.Error(err), zap.Int32("key_id", info.KeyID), zap.Int32("channel_id", info.ChannelID), zap.Int32("model_id", info.ModelID), zap.String("model", info.Model))
		return `{"request":null}`
	}
	return string(encoded)
}
