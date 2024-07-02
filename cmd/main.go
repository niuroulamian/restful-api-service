// Package main implements external-api service command
package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/emma-sleep/go-telemetry/mlog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"go.uber.org/zap"

	"github.com/niuroulamian/restful-api-service/config"
	"github.com/niuroulamian/restful-api-service/internal/app"
)

var version string
var rootCmd = &cobra.Command{
	Use:               "service",
	Short:             "template for go api service",
	Version:           version,
	PersistentPreRunE: initConfig,
	RunE:              run,
}

func main() {
	rootCmd.PersistentFlags().Int("log-level", 4, "debug=5, info=4, error=2, fatal=1, panic=0")

	if err := viper.BindPFlag("general.log_level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		zap.S().Fatal(err)
	}
	if err := rootCmd.Execute(); err != nil {
		zap.S().With(zap.Error(err)).Error("service exited with an error")
		os.Exit(-1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	var cfg app.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("couldn't unmarshal config file: %v", err)
	}
	cfg.Version = version

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	logger := mlog.NewWithOptions(&mlog.Options{
		Output: os.Stdout,
		Source: false,
		Level:  mlog.LevelDebug,
	})

	logger.Info(ctx, "starting service")
	a := app.New(cfg, logger)

	a.Start(ctx)

	<-a.Done()
	return fmt.Errorf("application has exited")
}

func initConfig(cmd *cobra.Command, args []string) error {
	// load /etc/service/service.toml if exists
	if _, err := os.Stat("/etc/service/service.toml"); err == nil {
		// config file exists, load file's content
		zap.S().Info("load configuration from /etc/service/service.toml")
		viper.SetConfigFile("/etc/service/service.toml")
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("couldn't read config file: %v", err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// file /etc/service/service.toml does not exist,
	// generate configuration content from service.tmpl
	viper.SetEnvPrefix("")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	buff := bytes.NewBuffer(nil)
	err := config.GetConfig(buff)
	if err != nil {
		return fmt.Errorf("couldn't get config: %v", err)
	}
	return viper.ReadConfig(buff)
}
