package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/conductorone/baton-gitlab/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var version = "dev"

func getConfigDir(name string) string {
	return filepath.Join(os.Getenv("PROGRAMDATA"), "ConductorOne", name)
}
func main() {
	ctx := context.Background()

	connectorName := "baton-gitlab"

	configPath := os.Getenv("BATON_CONFIG_PATH")
	if configPath == "" && os.Getenv("PROGRAMDATA") != "" {
		// Set BATON_CONFIG_PATH so that if we're running as a windows service, we use the correct config file
		err := os.Setenv("BATON_CONFIG_PATH", filepath.Join(getConfigDir(connectorName), "config.yaml"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
	_, cmd, err := config.DefineConfiguration(
		ctx,
		connectorName,
		getConnector,
		field.Configuration{
			Fields: ConfigurationFields,
		},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	cmd.Version = version

	err = cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func getConnector(ctx context.Context, v *viper.Viper) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)
	if err := ValidateConfig(v); err != nil {
		return nil, err
	}

	cb, err := connector.New(
		ctx,
		v.GetString(AccessToken.FieldName),
		v.GetString(BaseURL.FieldName),
		v.GetString(AccountCreationGroup.FieldName),
	)

	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}
	connector, err := connectorbuilder.NewConnector(ctx, cb)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}
	return connector, nil
}
