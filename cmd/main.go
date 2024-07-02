// Package main implements external-api service command
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/emma-sleep/go-telemetry/mlog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/niuroulamian/restful-api-service/internal/app"
)

var version string
var rootCmd = &cobra.Command{
	Use:     "service",
	Short:   "template for go api service",
	Version: version,
	RunE:    run,
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
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	logger := mlog.NewWithOptions(&mlog.Options{
		Output: os.Stdout,
		Source: false,
		Level:  mlog.LevelDebug,
	})

	logger.Info(ctx, "starting service")
	a := app.New(app.Config{}, logger)

	a.Start(ctx)

	<-a.Done()
	return fmt.Errorf("application has exited")
}
