package usage

type Usage struct {
	PromptTokens          int64
	CompletionTokens      int64
	CachedTokens          int64
	ThoughtTokens         int64
	ToolUseTokens         int64
	TotalTokens           int64
	InputTokensByModality []ModalityTokenUsage
}

type ModalityTokenUsage struct {
	Modality string
	Tokens   int64
}
