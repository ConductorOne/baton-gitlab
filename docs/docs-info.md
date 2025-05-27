While developing the connector, please fill out this form. This information is needed to write docs and to help other users set up the connector.

## Connector capabilities

1. What resources does the connector sync?
- Users
- Groups
- Projects

2. Can the connector provision any resources? If so, which ones?
This connector supports 
- Account Provisioning
- Entitlement Provisioning for Groups
- Entitlement Provisioning for Projects

## Connector credentials

1. What credentials or information are needed to set up the connector? (For example, API key, client ID and secret, domain, etc.)

TODO: BB-552

2. For each item in the list above:

    * How does a user create or look up that credential or info? Please include links to (non-gated) documentation, screenshots (of the UI or of gated docs), or a video of the process.

    * Does the credential need any specific scopes or permissions? If so, list them here.

    * If applicable: Is the list of scopes or permissions different to sync (read) versus provision (read-write)? If so, list the difference here.

    * What level of access or permissions does the user need in order to create the credentials? (For example, must be a super administrator, must have access to the admin console, etc.)  

### Notes about account provisioning
In order to support account provisioning `account-creation-group` is required. Any new user created from the connector will be a member of this group.
Be careful, the new user will inherit access as Guest to all Projects on this Group
