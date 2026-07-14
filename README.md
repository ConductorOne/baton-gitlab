![Baton Logo](./baton-logo.png)

# `baton-gitlab` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-gitlab.svg)](https://pkg.go.dev/github.com/conductorone/baton-gitlab) ![ci](https://github.com/conductorone/baton-gitlab/actions/workflows/ci.yaml/badge.svg)

`baton-gitlab` is a connector for  [GitLab](https://gitlab.com/) built using the [Baton SDK](https://github.com/conductorone/baton-sdk).

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

## Prerequisites

To use this connector, you will need different things depending on which version you want to use (DC-SaaS):

- SaaS version (Cloud): you need an API key with the api scope enabled, which is indicated by the `--access-token` flag and a group already created in gitlab for account creation and synchronization with the `--account-creation-group` flag.
  For connecting to https://gitlab.com you should do:
```
            baton-gitlab --access-token abcdefghij1234567890 --account-creation-group example-group
```

- DC version (on-premise/self-hosted): you need an API key with the api scope enabled, which is indicated by the `--access-token` flag and a base url with the `--base-url` flag.
  For connecting to https://example.local you should do:
```bash
            baton-gitlab --access-token abcdefghij1234567890 --base-url https://example.local
```

## Connector capabilities
- Sync Users, projects and groups.

- Optional access-path labeling (opt-in via `--sync-access-paths`, off by default):
  when enabled, group/project access is surfaced as distinct, reviewable paths —
  direct membership, inheritance from a parent/top-level group, and access via an
  invited (shared) group — instead of a flattened effective list. When disabled
  (the default), effective membership is emitted as flattened direct grants,
  preserving the previous grant shape and counts. The `--sync-direct-members-only`
  flag is separate and, in either mode, restricts syncing to direct memberships
  only (excluding all inherited/shared access).

- Supports Account provisioning:
  When you creating and new account, the following fields are required:
    - Name: The name of the user.
      Example: Name Example
    - Email Address: The user email address.
      Example: email@example.com
    - Username: The username to be used by the user.
      Example: Username Example

  IMPORTANT NOTE: Account provisioning is different for the DC and Cloud versions:
  -DC version (on-premise/self-hosted)= A separate user will be created to which entitlements can be assigned and revoked.
  -Cloud version= An invitation to the user's email address will be created, if the user has a gitlab account, in the next synchronization the new account will be automatically added,
  otherwise if the user does not have a gitlab account, a pending invitation resource will be created until the user creates a gitlab account.
  When the account is created, it will always be added to the group that is assigned in the flag.

- Supports Entitlements provisioning

- Supports User usage only for the DC version (on-premise/self-hosted)

- NOTE: in the cloud version, it is not possible to obtain the data of the attributes mail and last login of the users,
  because admin permissions are needed and in the cloud version they do not exist.
  https://docs.gitlab.com/api/users/

## Where can I find my API Key?
1- Log in gitlab.com o in your base url, then go to the top left, click on the user emoticon, a popup menu will open, click on edit profile.
2- In the dashboard to the left of User settings, click on access tokens, and in the list of tokens that appears, click on add new token.
3- In the new token creation options it is very important in select scopes to set the api item to active.
4- Add a name to the token, and create.

# Getting Started

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-gitlab
baton-gitlab
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_DOMAIN_URL=domain_url -e BATON_API_KEY=apiKey -e BATON_USERNAME=username ghcr.io/conductorone/baton-gitlab:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-gitlab/cmd/baton-gitlab@main

baton-gitlab

baton resources
```

# Data Model

`baton-gitlab` will pull down information about the following resources:
- Users
- Groups
- Projects

`baton-gitlab` supports account creation and entitlement provisioning for following resources:
- Groups
- Projects

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually
building spreadsheets. We welcome contributions, and ideas, no matter how
small&mdash;our goal is to make identity and permissions sprawl less painful for
everyone. If you have questions, problems, or ideas: Please open a GitHub Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-gitlab` Command Line Usage

```
baton-gitlab

Usage:
  baton-gitlab [flags]
  baton-gitlab [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  config             Get connector config
  help               Help about any command

Flags:
      --access-token string              required: The access token to authenticate with the GitLab API ($BATON_ACCESS_TOKEN)
      --account-creation-group string    The group indicated will be used as a default group for the new users. Required for account creation capability. ($BATON_ACCOUNT_CREATION_GROUP)
      --base-url string                  The base URL of the GitLab instance ($BATON_BASE_URL) (default "https://gitlab.com/")
      --client-id string                 The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string             The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
  -f, --file string                      The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                             help for baton-gitlab
      --log-format string                The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string                 The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
      --otel-collector-endpoint string   The endpoint of the OpenTelemetry collector to send observability data to ($BATON_OTEL_COLLECTOR_ENDPOINT)
  -p, --provisioning                     This must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --skip-full-sync                   This must be set to skip a full sync ($BATON_SKIP_FULL_SYNC)
      --sync-access-paths                Label grants by access path (direct, inherited, or invited group). Disabled (default) keeps the previous flattened effective-membership grants. ($BATON_SYNC_ACCESS_PATHS)
      --sync-direct-members-only         When enabled, only direct members of groups and projects are synced. Access inherited from parent groups or granted via invited (shared) groups is excluded. ($BATON_SYNC_DIRECT_MEMBERS_ONLY)
      --ticketing                        This must be set to enable ticketing support ($BATON_TICKETING)
  -v, --version                          version for baton-gitlab

Use "baton-gitlab [command] --help" for more information about a command.
```
