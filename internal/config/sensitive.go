package config

type SensitiveDictionaryConfig struct {
	Name            string   `mapstructure:"name"`
	EffectModels    []string `mapstructure:"effectModels"`
	KeywordFileList []string `mapstructure:"keywordFileList"`
}
