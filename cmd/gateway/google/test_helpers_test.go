package google

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	"github.com/HyperToken-dev/fabric/cmd/gateway/common"
)

type ModelInfo = common.ModelInfo
type UsageContext = common.UsageContext

const (
	ModelStatusActive = common.ModelStatusActive
	ModelStatusBanned = common.ModelStatusBanned
)

type fakeModelStore struct {
	model *common.ModelInfo
	err   error
}

func (s fakeModelStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*common.ModelInfo, error) {
	return s.model, s.err
}

type rejectingPolicy struct{}

func (rejectingPolicy) Detect(ctx context.Context, model, text string) sensitive.Result {
	if strings.TrimSpace(text) == "" {
		return sensitive.Result{}
	}
	return sensitive.Result{Matches: []sensitive.Match{{Dictionary: "test", Words: []string{"blocked"}}}}
}

type recordedIntegralLog struct {
	keyID    int32
	context  string
	response string
}

type recordingIntegralLogHandler struct {
	ch chan recordedIntegralLog
}

func (h *recordingIntegralLogHandler) InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error {
	h.ch <- recordedIntegralLog{keyID: keyID, context: context, response: response}
	return nil
}

func receiveIntegralLog(t *testing.T, ch <-chan recordedIntegralLog) recordedIntegralLog {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for integral log")
		return recordedIntegralLog{}
	}
}
