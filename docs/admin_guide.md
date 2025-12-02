# Admin Guide

This guide provides instructions for using the admin CLI to manage users and complaints.

## Commands

### `ban`

Bans a user.

**Usage:** `go run cmd/admin/main.go ban <user_id> [duration_in_hours]`

-   `<user_id>`: The ID of the user to ban.
-   `[duration_in_hours]` (optional): The duration of the ban in hours. If not provided, the ban will be permanent.

### `unban`

Unbans a user.

**Usage:** `go run cmd/admin/main.go unban <user_id>`

-   `<user_id>`: The ID of the user to unban.

### `confirm-complaint`

Confirms a complaint and rewards the reporter.

**Usage:** `go run cmd/admin/main.go confirm-complaint <complaint_id>`

-   `<complaint_id>`: The ID of the complaint to confirm.

## SQL Queries

### List Users with Lowest Reputation

```sql
SELECT id, reputation_score
FROM users
ORDER BY reputation_score ASC
LIMIT 10;
```

### Find Most Reported Users

```sql
SELECT suspect_id, COUNT(*) as complaint_count
FROM complaints
GROUP BY suspect_id
ORDER BY complaint_count DESC
LIMIT 10;
```
