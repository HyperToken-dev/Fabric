package models

import (
	"sort"
	"strings"
)

const (
	APIFormatOpenAI int32 = 1
	ModelTypeText   int32 = 1
)

type CatalogModel struct {
	Name string
	Type int32
}

var openAI = map[string]CatalogModel{
	"babbage-002":                {Name: "babbage-002", Type: ModelTypeText},
	"chat-latest":                {Name: "chat-latest", Type: ModelTypeText},
	"davinci-002":                {Name: "davinci-002", Type: ModelTypeText},
	"gpt-3.5-turbo":              {Name: "gpt-3.5-turbo", Type: ModelTypeText},
	"gpt-3.5-turbo-16k":          {Name: "gpt-3.5-turbo-16k", Type: ModelTypeText},
	"gpt-3.5-turbo-instruct":     {Name: "gpt-3.5-turbo-instruct", Type: ModelTypeText},
	"gpt-4.1":                    {Name: "gpt-4.1", Type: ModelTypeText},
	"gpt-4.1-mini":               {Name: "gpt-4.1-mini", Type: ModelTypeText},
	"gpt-4.1-nano":               {Name: "gpt-4.1-nano", Type: ModelTypeText},
	"gpt-4o":                     {Name: "gpt-4o", Type: ModelTypeText},
	"gpt-4o-mini":                {Name: "gpt-4o-mini", Type: ModelTypeText},
	"gpt-4o-mini-search-preview": {Name: "gpt-4o-mini-search-preview", Type: ModelTypeText},
	"gpt-4o-search-preview":      {Name: "gpt-4o-search-preview", Type: ModelTypeText},
	"gpt-5":                      {Name: "gpt-5", Type: ModelTypeText},
	"gpt-5-chat-latest":          {Name: "gpt-5-chat-latest", Type: ModelTypeText},
	"gpt-5-codex":                {Name: "gpt-5-codex", Type: ModelTypeText},
	"gpt-5-mini":                 {Name: "gpt-5-mini", Type: ModelTypeText},
	"gpt-5-nano":                 {Name: "gpt-5-nano", Type: ModelTypeText},
	"gpt-5-pro":                  {Name: "gpt-5-pro", Type: ModelTypeText},
	"gpt-5-search-api":           {Name: "gpt-5-search-api", Type: ModelTypeText},
	"gpt-5.1":                    {Name: "gpt-5.1", Type: ModelTypeText},
	"gpt-5.1-chat-latest":        {Name: "gpt-5.1-chat-latest", Type: ModelTypeText},
	"gpt-5.1-codex":              {Name: "gpt-5.1-codex", Type: ModelTypeText},
	"gpt-5.1-codex-max":          {Name: "gpt-5.1-codex-max", Type: ModelTypeText},
	"gpt-5.1-codex-mini":         {Name: "gpt-5.1-codex-mini", Type: ModelTypeText},
	"gpt-5.2":                    {Name: "gpt-5.2", Type: ModelTypeText},
	"gpt-5.2-chat-latest":        {Name: "gpt-5.2-chat-latest", Type: ModelTypeText},
	"gpt-5.2-codex":              {Name: "gpt-5.2-codex", Type: ModelTypeText},
	"gpt-5.2-pro":                {Name: "gpt-5.2-pro", Type: ModelTypeText},
	"gpt-5.3-chat-latest":        {Name: "gpt-5.3-chat-latest", Type: ModelTypeText},
	"gpt-5.3-codex":              {Name: "gpt-5.3-codex", Type: ModelTypeText},
	"gpt-5.4":                    {Name: "gpt-5.4", Type: ModelTypeText},
	"gpt-5.4-mini":               {Name: "gpt-5.4-mini", Type: ModelTypeText},
	"gpt-5.4-nano":               {Name: "gpt-5.4-nano", Type: ModelTypeText},
	"gpt-5.4-pro":                {Name: "gpt-5.4-pro", Type: ModelTypeText},
	"gpt-5.5":                    {Name: "gpt-5.5", Type: ModelTypeText},
	"gpt-5.5-pro":                {Name: "gpt-5.5-pro", Type: ModelTypeText},
	"o1":                         {Name: "o1", Type: ModelTypeText},
	"o3":                         {Name: "o3", Type: ModelTypeText},
	"o3-mini":                    {Name: "o3-mini", Type: ModelTypeText},
	"o4-mini":                    {Name: "o4-mini", Type: ModelTypeText},
}

func Lookup(apiFormat int32, modelName string) (CatalogModel, bool) {
	if apiFormat != APIFormatOpenAI {
		return CatalogModel{}, false
	}

	model, ok := openAI[strings.TrimSpace(modelName)]
	return model, ok
}

func List(apiFormat int32) []CatalogModel {
	if apiFormat != APIFormatOpenAI {
		return nil
	}

	models := make([]CatalogModel, 0, len(openAI))
	for _, model := range openAI {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})
	return models
}

func IsRestrictedAPIFormat(apiFormat int32) bool {
	return apiFormat == APIFormatOpenAI
}
