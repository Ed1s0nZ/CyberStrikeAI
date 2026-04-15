# Web User RBAC Smoke Test

1. Start the server with a valid `auth.password` configured in `config.yaml`.
2. Sign in as `admin` with that password and confirm the user menu shows `admin`.
3. Open `Settings` -> `Security` -> `Web access roles`, then create a role that only grants `system.config.read`.
4. Open `Settings` -> `Security` -> `Web users`, create a new Web user, and bind it to the read-only role.
5. Sign out, then sign in as the new Web user and confirm:
   - `GET /api/config` succeeds through the settings page load
   - write actions such as applying config or managing Web users return `403`
6. Sign back in as `admin`, disable that Web user, and confirm the old bearer token can no longer pass `/api/auth/validate`.
7. Reset that user's password, sign in with the new password, and confirm the user is forced to change password on the next login flow if applicable.
8. Open the separate `Roles` page and confirm existing AI Agent roles are unchanged by the Web RBAC operations above.
