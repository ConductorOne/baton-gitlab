package main

import (
	cfg "github.com/conductorone/baton-gitlab/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
)

func main() {
	config.Generate("gitlab", cfg.Config)
}
