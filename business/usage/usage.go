package usage

import "context"

type Record struct {
	KeyID            int32
	ChannelID        int32
	ModelID          int32
	Model            string
	Provider         string
	PromptTokens     int64
	CompletionTokens int64
}

type Context struct {
	KeyID     int32
	ChannelID int32
	ModelID   int32
	Model     string
}

type Recorder interface {
	RecordUsage(ctx context.Context, record Record) error
}

type NoopRecorder struct{}

func (NoopRecorder) RecordUsage(ctx context.Context, record Record) error {
	return nil
}
