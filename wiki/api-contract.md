# API contract and operational limits

Swagger is generated from the annotations in the Go handlers with `make swag`. This page records behavior that is important to operation and security but cannot be expressed completely by Swagger 2.0.

## Authentication

Send both user tokens and write-only device tokens as:

```http
Authorization: Bearer TOKEN
```

User Bearer Tokens can use configuration APIs and the data operations allowed by their namespace grant. WriteAccessTokens can only call `POST /api/v1/data/{namespace}` for the namespace that issued the token. Bearer tokens and OTPs are stored as SHA-256 hashes; newly issued Bearer credentials are returned only at creation, and legacy plaintext Bearer records are migrated to a hash after successful authentication.

Login and registration codes contain six decimal digits. Login codes expire after 10 minutes and registration codes after 30 minutes. Issuing a new code invalidates older codes for the same user or email, and verification consumes a code atomically so it cannot be reused.

Authentication endpoints are limited independently per source IP and per normalized email address in each application process:

| Endpoint | Window | Requests |
| --- | ---: | ---: |
| `POST /auth/login` | 10 minutes | 5 |
| `POST /auth/login/callback` | 10 minutes | 5 |
| `POST /auth/register` | 10 minutes | 3 |
| `POST /auth/register/callback` | 10 minutes | 5 |

Limits return `429 Too Many Requests`. A multi-instance production deployment must configure a shared Fiber limiter storage; the default in-memory limiter does not aggregate counts across instances.

## Namespace and token creation

`POST /cfg/me/namespace/create` returns `201 Created`, a `Location` header, and:

```json
{
  "namespace_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "grant_type": "admin"
}
```

WriteAccessToken creation returns a numeric `id` and the secret `token`. Store the secret at this point because it cannot be retrieved later. Revoke it without putting the secret in a URL:

```http
DELETE /api/v1/cfg/{namespace}/tokens/{token_id}
```

Namespace grants have these exact meanings:

| Grant | Read | Write | Manage users and tokens |
| --- | --- | --- | --- |
| `r` | yes | no | no |
| `w` | no | yes | no |
| `rw` | yes | yes | no |
| `admin` | yes | yes | yes |

The database enforces one grant for each namespace/user pair.

On upgrade, startup adds the new hash/audit columns and uniqueness indexes without running a full DuckDB table alteration. If an older database already contains duplicate namespace/user grants, reconcile those rows before upgrading; migration deliberately fails instead of choosing an arbitrary permission.

## Data limits and atomicity

The HTTP body limit is 1 MiB. A write batch must contain 1–1000 total key/value items. Each group requires a Unix timestamp in seconds and at least one item. Keys are 1–128 characters, values are required but may be empty, and a value is at most 65,536 bytes.

Invalid JSON returns `400`, semantic validation errors return `422`, and an oversized body returns `413`. A batch is stored in one transaction: either every item succeeds or none is persisted.

Reads default to 50 records and accept 1–50. Offset pagination remains available for compatibility, but clients reading a changing time series should pass the response's `next_cursor` back as `cursor`; cursor and non-zero offset cannot be combined. Ordering uses time and record ID so equal timestamps are stable.

## Logging

Access logs record the authenticated user ID or WriteAccessToken ID as the actor. OTPs, tokens, passwords, and KVS request payloads are not copied to the access-log body. Application and database log retention and access control remain deployment responsibilities.
