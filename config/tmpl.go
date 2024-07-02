// Package config provides template for toml file used as configuration for this service.
/*
When service.toml is not provided, service will call GetConfig to compose configuration file from env variables and template string
configTmplStr.
Note that variable names in configTmplStr should follow the specified mapstructure value in definition of app.Config.
*/
package config

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/spf13/viper"

	"go.uber.org/zap"
)

var configTmplStr = `
[logging]
level = "{{getenv "LOG_LEVEL"}}"
encode_json = false
[logging.sampling]
interval_seconds = 10
first = 10
thereafter = 100
`

func getEnv(key string) (interface{}, error) {
	param, ok := os.LookupEnv(key)
	if !ok {
		return nil, fmt.Errorf("env variable %s must be defined", key)
	}
	return param, nil
}

// GetConfig takes in configuration template, search in local env variables with same key word, then generate configuration
func GetConfig(buff *bytes.Buffer) error {
	zap.S().Info("load configuration from configTmplStr")
	name := "service.toml"
	configTemp, err := template.New(name).Funcs(template.FuncMap{
		"getenv": getEnv,
	}).Parse(configTmplStr)
	if err != nil {
		return fmt.Errorf("couldn't parse template : %v", err)
	}
	if err := configTemp.ExecuteTemplate(buff, name, nil); err != nil {
		return fmt.Errorf("couldn't execute template %s: %v", name, err)
	}
	viper.SetConfigType("toml")
	return nil
}
