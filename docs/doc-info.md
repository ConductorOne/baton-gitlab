While developing the connector, please fill out this form. This information is needed to write docs and to help other users set up the connector.

## Connector capabilities
- Sync Users, projects and groups.

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

1. What resources does the connector sync?
   This connector syncs:
   — Users
   — Projects
   — Groups

2. Can the connector provision any resources? If so, which ones?
   The connector can provision:
   — Group entitlements for Users 
   — Project entitlements for Users

## Access-path labeling (--sync-access-paths, opt-in)

By default the connector emits effective membership as flattened direct grants: a
user who can access a group/project shows up as a single grant, regardless of
whether that access is direct, inherited from a parent group, or granted through an
invited (shared) group. All paths look the same.

When `--sync-access-paths` is enabled, each way a principal obtained access is
surfaced as its own expandable grant, so the access path is visible and reviewable:

  - Direct membership — a plain user grant on the resource's access-level entitlement.
  - Inheritance from a parent/top-level group — one expandable grant per ancestor
    group and per access level that ancestor actually has members at, with the
    ancestor group as principal, expanding into that group's matching per-level
    entitlement. Each ancestor (immediate subgroup up to the top-level group) is a
    distinct path, so a top-level group member is shown as having access to nested
    projects through the top-level group specifically.
  - Access via an invited (shared) group — the invited group as principal on the
    target, expanding into the invited group's per-level entitlements. GitLab caps a
    shared member's effective access at min(their level in the invited group, the
    share's access level); the connector applies that cap per level, so no member is
    shown at a higher level than GitLab actually grants.

Design notes:
  - Expandable grants use Shallow=true: each grant is a single, crisp hop, and the
    ConductorOne platform resolves deeper nesting by walking the connected edges.
  - No new entitlements are added — the model reuses the existing per-access-level
    entitlements, and only emits paths for access levels that actually have members,
    so no grant expands to an empty set.
  - The default (flag off) output is unchanged (identical grant shape and counts),
    so existing deployments are unaffected until they opt in.
  - `--sync-direct-members-only` still applies: with it set, only direct members are
    synced and the inheritance/invited paths are not emitted.

Known limitation: GitLab group→project sharing also grants access to the invited
group's inherited members. When the invited group is itself a subgroup, those
inherited members are surfaced via their own group's path rather than the invited
path. Invited top-level groups (the common case) are unaffected, since their direct
and effective membership are the same.

## Connector credentials

1. What credentials or information are needed to set up the connector? (For example, API key, client ID and secret, domain, etc.)
    - DC version (on-premise/self-hosted): Requires an API key and a base url. Args: --access-token and --base-url
    - Cloud version: Requires an API key for synchronization. To enable account provisioning or deprovisioning, you must also provide the name of an existing GitLab group — only users within this group can be added or removed. 
      Use the --access-token and --account-creation-group arguments


2. For each item in the list above:

    * How does a user create or look up that credential or info? Please include links to (non-gated) documentation, screenshots (of the UI or of gated docs), or a video of the process.
      
      DC version (on-premise/self-hosted): 1- In order to have your own account you must first create a local instance of gitlab in which you can configure the root user and the url base that will be used to log in.
                                           2- Once the url base is configured, it can be used to login with the root user data, and also for the --base-url flag.
                                           3- To get the apikey, log in to the local url, go to the top left, click on the user emoticon, a popup menu will open, click on edit profile.
                                           4- In the dashboard to the left of User settings, click on access tokens, and in the list of tokens that appears, click on add new token.
                                           5- In the new token creation options it is very important in select scopes to set the api item to active.
                                           6- Add a name to the token, and create.

     Cloud version: API key: 1- Log in gitlab.com, then go to the top left, click on the user emoticon, a popup menu will open, click on edit profile.
                             2- In the dashboard to the left of User settings, click on access tokens, and in the list of tokens that appears, click on add new token.
                             3- In the new token creation options it is very important in select scopes to set the api item to active.
                             4- Add a name to the token, and create.
                    Group: 1- Login, in the left dashboard click on groups, a page will be displayed showing the groups you have in the account, copy the name of the desired group for the flag.
                              In case you do not have a group, in the same page, click on the top right on create a new group. Assign it a name which you will then use as in step 1.

  * Does the credential need any specific scopes or permissions? If so, list them here.
    The api key in both versions must always have the api option active in the scopes.
    You can configure it at the moment you create the new apikey.

  * If applicable: Is the list of scopes or permissions different to sync (read) versus provision (read-write)? If so, list the difference here.
    In order to do both, you need to have the api option active in the api token scopes.

  * What level of access or permissions does the user need in order to create the credentials? (For example, must be a super administrator, must have access to the admin console, etc.)  
    In the dc version you need to log in with a user with admin permissions and in the cloud version with your normal account is enough. 