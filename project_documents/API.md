# GoodSpot247 Backend — API Reference

Base URL: `http://localhost:8081`

All timestamps are RFC 3339 / ISO 8601 (`2026-09-10T14:13:19.321721Z`).

Auth uses a JWT bearer token (30-day expiry). Supply it as:

```
Authorization: Bearer <token>
```

---

## Table of Contents

| Method | Path                    | Auth                              | Description                              |
| ------ | ----------------------- | --------------------------------- | ---------------------------------------- |
| GET    | `/health`               | none                              | Liveness + DB connectivity check         |
| POST   | `/auth/register`        | none (rate-limited)               | Create account, returns JWT              |
| POST   | `/auth/login`           | none (rate-limited)               | Authenticate, returns JWT                |
| GET    | `/me`                   | Bearer token                      | Current user ID + role from token        |
| GET    | `/users/:id`            | none                              | Public user profile                      |
| PATCH  | `/users/:id/role`       | Bearer token + **owner**          | Promote/demote user role                 |
| GET    | `/categories`           | none                              | List pin categories                      |
| GET    | `/pins`                 | none                              | List pins in a bounding box              |
| GET    | `/pins/:id`             | none                              | Pin detail with ordered photos           |
| POST   | `/pins`                 | Bearer token (rate-limited)       | Create a pin with photo uploads          |
| POST   | `/pins/:id/report`      | Bearer token                      | Report a pin                             |
| PATCH  | `/reports/:id`          | Bearer token + **admin**          | Approve or dismiss a report              |
| GET    | `/uploads/*`            | none                              | Static photo files                       |

**Roles:** `user` < `admin` < `owner`. `RequireAdmin` allows `admin` and `owner`.
`RequireOwner` allows only `owner`. Role is re-validated against the DB on every request.

**Rate limits** (per IP unless noted):

| Endpoint     | Limit                  |
| ------------ | ---------------------- |
| POST /auth/register | 5 / minute      |
| POST /auth/login    | 20 / minute (IP) + 5 / minute (email) |
| POST /pins          | 10 / minute      |

---

## Authentication

### POST `/auth/register`

Create a new account. Email is stored lowercased. Rate limit: 5/min per IP.

Request (JSON):

| Field    | Type   | Required | Constraints          |
| -------- | ------ | -------- | -------------------- |
| email    | string | yes      | valid email          |
| password | string | yes      | 8–72 chars           |
| username | string | yes      | 3–30 chars           |

Example request:

```json
{
  "email": "user@example.com",
  "password": "password123",
  "username": "cooluser"
}
```

Responses:

- `201 Created` — new user + JWT
  ```json
  {
    "user": {
      "id": "966e7776-82b3-4096-b310-d41cae451a9b",
      "email": "user@example.com",
      "username": "cooluser",
      "avatar_url": null,
      "socials": {},
      "role": "user",
      "created_at": "2026-09-10T12:00:00Z",
      "updated_at": "2026-09-10T12:00:00Z"
    },
    "token": "eyJhbGciOi..."
  }
  ```
- `400 Bad Request` — invalid body/missing field (`{"error": "..."}`)
- `409 Conflict` — `email already registered` / `username already taken`
- `429 Too Many Requests` — `too many requests, try again later`

---

### POST `/auth/login`

Authenticate. Rate limits: 20/min per IP + 5/min per email.

Request (JSON):

| Field    | Type   | Required |
| -------- | ------ | -------- |
| email    | string | yes      |
| password | string | yes      |

Responses:

- `200 OK` — same shape as `/auth/register` (`user` + `token`)
- `400 Bad Request` — invalid body
- `401 Unauthorized` — `invalid email or password` (generic; does not reveal which)
- `429 Too Many Requests` — `too many requests, try again later`

---

### GET `/me`

Return caller identity from the validated token.

Responses:

- `200 OK`
  ```json
  { "user_id": "966e7776-82b3-4096-b310-d41cae451a9b", "role": "user" }
  ```
- `401 Unauthorized` — `missing or invalid Authorization header` / `invalid or expired token`

---

## Users

### GET `/users/:id`

Public profile. Never exposes `password_hash`.

Responses:

- `200 OK` — `PublicUser` (no `updated_at`)
  ```json
  {
    "id": "966e7776-82b3-4096-b310-d41cae451a9b",
    "email": "user@example.com",
    "username": "cooluser",
    "avatar_url": null,
    "socials": {},
    "role": "user",
    "created_at": "2026-09-10T12:00:00Z"
  }
  ```
- `404 Not Found` — `user not found`

---

### PATCH `/users/:id/role`

**Auth: Bearer token + owner role.** Promote/demote a user's role.

Request (JSON):

| Field | Type   | Required | Constraints      |
| ----- | ------ | -------- | ---------------- |
| role  | string | yes      | `user` or `admin`|

> Note: `owner` cannot be assigned via API. Last remaining owner cannot be
> demoted (guard: `cannot demote the last owner`).

Responses:

- `200 OK` — updated `PublicUser`
- `400 Bad Request` — invalid role or `cannot demote the last owner`
- `401 Unauthorized` — missing/invalid token
- `403 Forbidden` — `admin access required` style guard; `owner access required`
- `404 Not Found` — `user not found`

---

## Categories

### GET `/categories`

List all pin categories, order by id.

Responses:

- `200 OK` — array
  ```json
  [
    { "id": 1, "name": "Food" },
    { "id": 2, "name": "Nature" }
  ]
  ```

---

## Pins

### GET `/pins`

List visible (non-hidden) pins intersecting a bounding box. Public.

Query parameters:

| Param    | Type    | Required | Description                                        |
| -------- | ------- | -------- | -------------------------------------------------- |
| bbox     | string  | yes      | `minLat,minLng,maxLat,maxLng` (4 comma-separated floats) |
| category | integer | no       | Filter by category id                              |
| limit    | integer | no       | 1–200, default 200                                 |

Response items include the cover photo (first photo by `position`) and author
username. `location` is WKT, e.g. `"POINT(120.98 14.65)"`.

Responses:

- `200 OK`
  ```json
  {
    "pins": [
      {
        "id": "77ee36af-36f1-482a-81c9-e4b6e73c5646",
        "user_id": "966e7776-82b3-4096-b310-d41cae451a9b",
        "location": "POINT(120.98 14.65)",
        "geohash": "wdw52fy",
        "caption": null,
        "category_id": 1,
        "is_hidden": false,
        "created_at": "2026-09-10T14:14:10.24044Z",
        "cover_url": "http://localhost:8081/uploads/p5.webp",
        "username": "dave2"
      }
    ]
  }
  ```
- `400 Bad Request` — missing/malformed `bbox`, bad `category`, `limit` out of range
- Hidden pins (`is_hidden = true`, e.g. via an approved report) are never returned.

---

### GET `/pins/:id`

Pin detail with author + category + ordered `photos` array. Public.

Responses:

- `200 OK`
  ```json
  {
    "pin": {
      "id": "46cc4248-f869-416d-8bd8-f37bab64c130",
      "user_id": "966e7776-82b3-4096-b310-d41cae451a9b",
      "location": "POINT(120.9842 14.5995)",
      "geohash": "wdw511f",
      "caption": "Best food spot",
      "category_id": 1,
      "is_hidden": false,
      "created_at": "2026-09-10T14:13:19.321721Z",
      "category": "Food",
      "username": "dave2",
      "avatar_url": null,
      "photos": [
        {
          "id": "0f17abd7-6921-4296-bcf6-a6b0411470d4",
          "pin_id": "46cc4248-f869-416d-8bd8-f37bab64c130",
          "photo_url": "http://localhost:8081/uploads/p1.webp",
          "thumbnail_url": "http://localhost:8081/uploads/p1_thumb.webp",
          "position": 0,
          "created_at": "2026-09-10T14:13:19.321721Z"
        }
      ]
    }
  }
  ```
- `404 Not Found` — `pin not found`

---

### POST `/pins`

**Auth: Bearer token. Rate limit: 10/min per IP.**

Create a pin. Accepts `multipart/form-data` with real image files (Phase 3).

Form fields:

| Field       | Type      | Required | Constraints                          |
| ----------- | --------- | -------- | ------------------------------------ |
| lat         | string    | yes      | float, -90..90                       |
| lng         | string    | yes      | float, -180..180                     |
| category_id | string    | yes      | int, must exist in `/categories`     |
| caption     | string    | no       | free text                            |
| photos      | file(s)   | yes      | 1–5 files, jpg/png only, ≤10MB each, magic bytes validated |

Example (curl):

```bash
curl -X POST http://localhost:8081/pins \
  -H "Authorization: Bearer <token>" \
  -F "lat=14.63" -F "lng=120.99" \
  -F "caption=Delicious ramen" -F "category_id=1" \
  -F "photos=@/path/to/photo1.jpg" \
  -F "photos=@/path/to/photo2.png"
```

Responses:

- `201 Created` — pin (without photos array)
  ```json
  {
    "pin": {
      "id": "46cc4248-f869-416d-8bd8-f37bab64c130",
      "user_id": "966e7776-82b3-4096-b310-d41cae451a9b",
      "location": "POINT(120.99 14.63)",
      "geohash": "wdw511f",
      "caption": "Delicious ramen",
      "category_id": 1,
      "is_hidden": false,
      "created_at": "2026-09-10T14:13:19.321721Z"
    }
  }
  ```
- `400 Bad Request` — not multipart, missing/invalid `lat`/`lng`/`category_id`,
  no photos, >5 photos, photo >10MB, non-jpg/png file (`only jpg and png images are allowed`)
- `401 Unauthorized` — missing/invalid token
- `429 Too Many Requests` — rate limit

Photo URLs are `http://localhost:8081/uploads/<id>.webp` (downscaled to ≤1600px,
WebP q80) and are served by `GET /uploads/*`. A 400px square thumbnail is stored
alongside each photo (`thumbnail_url`) and is used as `cover_url` in `GET /pins`
list responses; originals are discarded after conversion.

---

## Reports

### POST `/pins/:id/report`

**Auth: Bearer token.** Report a pin. `reporter_id` is taken from the token.
One report per (pin, reporter) pair.

Request (JSON):

| Field  | Type   | Required | Constraints |
| ------ | ------ | -------- | ----------- |
| reason | string | yes      | ≥ 3 chars   |

Example:

```json
{ "reason": "Inappropriate content" }
```

Responses:

- `201 Created`
  ```json
  {
    "report": {
      "id": "79c5087f-1fc3-4922-a39b-c0191e8f54da",
      "pin_id": "c31a699c-b053-4750-9c0b-0bf22bcbcbdf",
      "reporter_id": "966e7776-82b3-4096-b310-d41cae451a9b",
      "reason": "Inappropriate content",
      "status": "pending",
      "resolved_by": null,
      "resolved_at": null,
      "created_at": "2026-09-10T14:55:56.360799Z"
    }
  }
  ```
- `400 Bad Request` — invalid body
- `401 Unauthorized` — missing/invalid token
- `404 Not Found` — `pin not found`
- `409 Conflict` — `you already reported this pin` (unique `pin_id`+`reporter_id`)

Status values: `pending` → `reviewed` | `actioned`.

---

### PATCH `/reports/:id`

**Auth: Bearer token + admin role.** Resolve a pending report. Only affects
reports currently in `pending` status (others → `409`).

Request (JSON):

| Field  | Type   | Required | Constraints           |
| ------ | ------ | -------- | --------------------- |
| action | string | yes      | `approve` or `dismiss`|

Behavior:

| Action   | Report status | Pin effect                |
| -------- | ------------- | ------------------------- |
| approve  | `actioned`    | `is_hidden = true`        |
| dismiss  | `reviewed`    | untouched                 |

In both cases `resolved_by` (from token) and `resolved_at` are set.

Responses:

- `200 OK` — updated report (`resolved_by`/`resolved_at` populated)
- `400 Bad Request` — invalid action
- `401 Unauthorized` — missing/invalid token
- `403 Forbidden` — non-admin caller
- `404 Not Found` — `report not found`
- `409 Conflict` — `report already resolved`

---

## Files

### GET `/uploads/*`

Serve uploaded photo files. Public. Path comes from the stored `photo_url`
(e.g. `http://localhost:8081/uploads/<id>.jpg`).

Redis (if used) and the R2 driver are not wired yet — storage is local-only
(`./uploads` directory in the server working directory).

---

## Error format

All errors return a JSON object with a single `error` key:

```json
{ "error": "human readable message" }
```