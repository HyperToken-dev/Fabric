package usage

import (
	"github.com/pkoukk/tiktoken-go"
	tiktoken_loader "github.com/pkoukk/tiktoken-go-loader"
)

func GetTextToken(text, encoding string) (int, error) {
	if encoding == "" {
		encoding = "cl100k_base"
	}
	tiktoken.SetBpeLoader(tiktoken_loader.NewOfflineLoader())
	tke, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return 0, err
	}
	token := tke.Encode(text, nil, nil)
	return len(token), nil
}
