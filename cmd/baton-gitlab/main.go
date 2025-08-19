package main

import (
	"context"
	"fmt"
	"os"

	cfg "github.com/conductorone/baton-gitlab/pkg/config"
	"github.com/conductorone/baton-gitlab/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := config.DefineConfiguration(
		ctx,
		"baton-gitlab",
		getConnector,
		cfg.Config,
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

func getConnector(ctx context.Context, glc *cfg.Gitlab) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)

	err := field.Validate(cfg.Config, glc)
	if err != nil {
		return nil, err
	}

	cb, err := connector.New(
		ctx,
		glc.AccessToken,
		glc.BaseUrl,
		glc.AccountCreationGroup,
		glc.SyncDirectMembersOnly,
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
