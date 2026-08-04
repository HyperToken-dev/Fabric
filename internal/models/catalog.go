package models

import (
	"sort"
	"strings"
)

const (
	APIFormatOpenAI         int32 = 1
	APIFormatAlibabaBailian int32 = 2
	APIFormatSeedance       int32 = 3
	APIFormatGoogle         int32 = 4
	ModelTypeText           int32 = 1
	ModelTypeVideo          int32 = 2
)

type CatalogModel struct {
	Name string
	Type int32
}

func Lookup(apiFormat int32, modelName string) (CatalogModel, bool) {
	catalog := catalogForAPIFormat(apiFormat)
	if catalog == nil {
		return CatalogModel{}, false
	}

	model, ok := catalog[strings.TrimSpace(modelName)]
	return model, ok
}

func List(apiFormat int32) []CatalogModel {
	catalog := catalogForAPIFormat(apiFormat)
	if catalog == nil {
		return nil
	}

	models := make([]CatalogModel, 0, len(catalog))
	for _, model := range catalog {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})
	return models
}

func IsRestrictedAPIFormat(apiFormat int32) bool {
	return catalogForAPIFormat(apiFormat) != nil
}

func catalogForAPIFormat(apiFormat int32) map[string]CatalogModel {
	switch apiFormat {
	case APIFormatOpenAI:
		return openAI
	case APIFormatAlibabaBailian:
		return alibabaBailian
	case APIFormatSeedance:
		return seedance
	case APIFormatGoogle:
		return google
	default:
		return nil
	}
}
