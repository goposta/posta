---
sidebar_position: 7
title: LDAP Directories
description: Let people sign in with their LDAP or Active Directory account (Posta Enterprise)
---

# LDAP Directories

:::info Posta Enterprise
LDAP / Active Directory sign-in needs a Posta Enterprise license, installed under
**Admin → License**. Without one, the directories you configured stay stored but
are not used.
:::

People sign in on the usual Posta sign-in page with their **directory email and
password**. Posta asks a directory only when the password does not match a local
account, so local accounts and the bootstrap admin keep working if the directory
is down.

The first time someone signs in, Posta creates their account (verified, with no
local password). An existing Posta account with the same email is used as is.

## Adding a directory

Under **Admin → LDAP Directories → Add directory**:

| Setting | Notes |
|---|---|
| Host, port | Port defaults to 389, or 636 for LDAPS. |
| Encryption | **StartTLS** (default), **LDAPS**, or **None** for development only. Add a private CA certificate in PEM if the server's certificate is not publicly trusted. |
| Service account | The bind DN and password Posta searches for users with. Leave empty to search anonymously. The password is encrypted at rest when `POSTA_ENCRYPTION_KEY` is set, and never returned by the API. |
| Users | The base DN to search, and the filter. `%s` is the sign-in email, escaped. The default is `(mail=%s)`; for Active Directory, `(userPrincipalName=%s)` or `(|(mail=%s)(userPrincipalName=%s))` are common. |
| Attributes | `mail` and `displayName` by default. |
| Groups | Posta reads the user's `memberOf` by default. Set a group base DN or a filter such as `(member=%s)` (`%s` is the user's DN) to search for groups instead, and tick **nested groups** for Active Directory. |

Then press **Test connection**: Posta connects, binds as the service account and
reads the user base DN.

Posta tries enabled directories in the order they were added and uses the first
one that knows the email.

## Group mappings

A mapping gives the members of a directory group access in Posta, at every sign-in:

- **Platform admin**: members become platform admins.
- **Workspace and role**: members join the workspace as admin, editor or viewer.
  Ownership is never granted from a directory.

Write the group as a full DN (`cn=posta-admins,ou=groups,dc=example,dc=com`) or as
a bare CN (`posta-admins`); matching ignores case.

Mappings apply only to accounts the directory created, and only to the access they
govern: a workspace no mapping names, or the platform admin role when no mapping
grants it, is left as an admin set it. Posta never takes the admin role from the
last platform admin who can sign in.

## API

All routes need a platform admin and a Posta Enterprise license; changes need one
that has not expired.

| Method | Path | |
|---|---|---|
| `GET` | `/api/v1/admin/ldap` | List directories with their mappings |
| `POST` | `/api/v1/admin/ldap` | Add a directory |
| `PUT` | `/api/v1/admin/ldap/{id}` | Update one; an empty `bind_password` keeps the stored one |
| `DELETE` | `/api/v1/admin/ldap/{id}` | Delete one and its mappings |
| `POST` | `/api/v1/admin/ldap/{id}/test` | Test the connection |
| `POST` | `/api/v1/admin/ldap/{id}/mappings` | Map a group |
| `DELETE` | `/api/v1/admin/ldap/{id}/mappings/{mapping_id}` | Remove a mapping |
