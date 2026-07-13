While developing the connector, please fill out this form. This information is needed to write docs and to help other users set up the connector.

## Connector capabilities
- Sync Users, projects and groups.

- Labels grants by access path: memberships are surfaced as distinct, reviewable
  paths — direct membership, inheritance from a parent/top-level group, and
  access through an invited (shared) group — instead of a flattened effective
  list. Indirect paths are expressed via expandable grants (group-as-principal
  pointing at a group's membership entitlement). The `--sync-direct-members-only`
  flag restricts syncing to direct memberships only.

  Invited (shared) group resolution follows GitLab's own sharing semantics,
  which differ by target: a group shared into another **group** grants access
  only to the invited group's *direct* members, so that path expands to the
  invited group's `member` entitlement; a group shared into a **project** grants
  access to the invited group's *direct and inherited* members, so that path
  expands to the invited group's `effective-member` entitlement (composed from
  the group's direct members plus its ancestor-inherited members via expansion,
  no extra API calls). Sharing is non-transitive — a group's own inbound (shared)
  members are not re-shared onward — so `effective-member` deliberately excludes
  them. Parent/top-level inheritance is fully transitive in both cases.

  Known limitation: on the invited (shared) group path, expanded members are
  shown at the share's `group_access_level`, which is an upper bound. GitLab caps
  a shared member's effective access at `min(role in the invited group,
  group_access_level)`, so a member whose role in the invited group is below the
  share level may be shown one level higher on the shared path. This is a
  consequence of expressing the path via expansion (`GrantExpandable` copies
  principals without recomputing per-member levels). The direct and
  parent/top-level inheritance paths preserve exact access levels; only the
  invited-group path uses the share level.

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