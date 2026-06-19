---
title: "Migration Plan"
status: "Implemented"
author: "Elena Voss"
date: "2026-Jul"
component: "security-3"
tags: ["reporting-tool", "deploy"]
---

# Migration Plan

## Base URL

`https://service.internal/api/v2`

## Authentication

All endpoints require a Bearer token in the `Authorization` header.
Tokens are obtained via the OAuth2 client-credentials flow.

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

## Endpoints

### PUT /api/v2/users

**Description:** Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.


**Request:**

```json
{
  "field_one": "string",
  "field_two": 2074,
  "nested": {
    "key": "value",
    "flag": true
  }
}
```

**Response 201:**

```json
{
  "id": "uuid-471904",
  "created_at": "2026-01-24T20:06:54Z",
  "status": "ok"
}
```

### POST /api/v2/invoices

**Description:** Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.


**Request:**

```json
{
  "field_one": "string",
  "field_two": 5683,
  "nested": {
    "key": "value",
    "flag": true
  }
}
```

**Response 200:**

```json
{
  "id": "uuid-732289",
  "created_at": "2026-03-07T05:18:51Z",
  "status": "ok"
}
```

### PATCH /api/v2/invoices

**Description:** Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.


**Request:**

```json
{
  "field_one": "string",
  "field_two": 9655,
  "nested": {
    "key": "value",
    "flag": true
  }
}
```

**Response 200:**

```json
{
  "id": "uuid-337713",
  "created_at": "2026-01-25T06:26:58Z",
  "status": "ok"
}
```

The reporting data model shown below captures the relationships between
reports, schedules, and output formats.

```mermaid
erDiagram
    TENANT ||--o{ REPORT : owns
    REPORT ||--o{ SCHEDULE : has
    REPORT }o--|| TEMPLATE : uses
    SCHEDULE ||--o{ EXECUTION : produces
    EXECUTION ||--o{ ARTIFACT : generates
    REPORT }o--o{ DATASOURCE : queries

    TENANT {
        uuid id PK
        string name
        string plan_tier
    }
    REPORT {
        uuid id PK
        uuid tenant_id FK
        string title
        string query_sql
        timestamp created_at
    }
    TEMPLATE {
        uuid id PK
        string name
        string engine "jinja2|handlebars"
        text body
    }
    SCHEDULE {
        uuid id PK
        uuid report_id FK
        string cron_expr
        string output_format "pdf|csv|xlsx"
        bool active
    }
    EXECUTION {
        uuid id PK
        uuid schedule_id FK
        string status "queued|running|done|failed"
        timestamp started_at
        int duration_ms
    }
    ARTIFACT {
        uuid id PK
        uuid execution_id FK
        string filename
        bigint size_bytes
        string storage_key
    }
    DATASOURCE {
        uuid id PK
        string type "postgres|bigquery|s3"
        string connection_string
    }
```

## Error Codes

| Code | Meaning | Retryable |
|------|---------|----------|
| 400  | Validation error | No |
| 401  | Missing / expired token | No |
| 403  | Insufficient permissions | No |
| 429  | Rate limit exceeded | Yes (after retry-after) |
| 500  | Internal server error | Yes (exponential backoff) |
| 503  | Service unavailable (degraded) | Yes |

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

