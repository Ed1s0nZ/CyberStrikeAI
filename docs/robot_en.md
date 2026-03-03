# CyberStrikeAI Robot / Chatbot Guide

[Chinese](robot.md)

This document explains how to chat with CyberStrikeAI from **DingTalk**, **Lark (Feishu)**, and **Telegram** using long-lived connections—no need to open a browser on the server. Following the steps below helps avoid common mistakes.

---

## 1. Where to configure in CyberStrikeAI

1. Log in to the CyberStrikeAI web UI.
2. Open **System Settings** in the left sidebar.
3. Click **Robot settings** (between “Basic” and “Security”).
4. Enable the platform and fill in credentials (DingTalk: Client ID / Client Secret; Lark: App ID / App Secret).
5. Click **Apply configuration** to save.
6. **Restart the CyberStrikeAI process** (saving alone does not establish the connection).

Settings are written to the `robots` section of `config.yaml`; you can also edit the file directly. **After changing DingTalk or Lark config, you must restart for the long-lived connection to take effect.**

---

## 2. Supported platforms (long-lived connection)

| Platform | Description |
|----------|-------------|
| DingTalk | Stream long-lived connection; the app connects to DingTalk to receive messages |
| Lark (Feishu) | Long-lived connection; the app connects to Lark to receive messages |
| Telegram | Long-polling connection; the app polls Telegram's Bot API for messages |

Section 3 below describes, per platform, what to do in the developer console and which fields to copy into CyberStrikeAI.

---

## 3. Configuration and step-by-step setup

### 3.1 DingTalk

**Important: two types of DingTalk bots**

| Type | Where it’s created | Can do “user sends message → bot replies”? | Supported here? |
|------|-------------------|-------------------------------------------|------------------|
| **Custom bot (Webhook)** | In a DingTalk group: Group settings → Add robot → Custom (Webhook) | No; you can only post to the group | No |
| **Enterprise internal app bot** | [DingTalk Open Platform](https://open.dingtalk.com): create an app and enable the bot | Yes | Yes |

If you only have a **custom bot** Webhook URL (`oapi.dingtalk.com/robot/send?access_token=...`) and sign secret (`SEC...`), **do not** put them into CyberStrikeAI. You must create an **enterprise internal app** in the open platform and obtain **Client ID** and **Client Secret** as below.

---

**DingTalk setup (in order)**

1. **Open DingTalk Open Platform**  
   Go to [https://open.dingtalk.com](https://open.dingtalk.com) and log in with an **enterprise admin** account.

2. **Create or select an app**  
   In the left menu: **Application development** → **Enterprise internal development** → **Create application** (or choose an existing app). Fill in the app name and create.

3. **Get Client ID and Client Secret**  
   - In the left menu open **Credentials and basic info** (under “Basic information”).  
   - Copy **Client ID (formerly AppKey)** and **Client Secret (formerly AppSecret)**.  
   - Use copy/paste; avoid typing by hand. Watch for **0** vs **o** and **1** vs **l** (e.g. `ding9gf9tiozuc504aer` has the digits **504**, not 5o4).

4. **Enable the bot and choose Stream mode**  
   - Left menu: **Application capabilities** → **Robot**.  
   - Turn on “Robot configuration”.  
   - Fill in robot name, description, etc. as required.  
   - **Critical**: set message reception to **”Stream mode”** (streaming access). If you only enable “HTTP callback” or do not select Stream, CyberStrikeAI will not receive messages.  
   - Save.

5. **Permissions and release**  
   - Left menu: **Permission management** — search for “robot”, “message”, etc., and enable **receive message**, **send message**, and other bot-related permissions; confirm.  
   - Left menu: **Version management and release** — if there are unpublished changes, click **Release new version** / **Publish**; otherwise changes do not take effect.

6. **Fill in CyberStrikeAI**  
   - In CyberStrikeAI: System settings → Robot settings → DingTalk.  
   - Enable “Enable DingTalk robot”.  
   - Paste the Client ID and Client Secret from step 3.  
   - Click **Apply configuration**, then **restart CyberStrikeAI**.

---

**Field mapping (DingTalk)**

| Field in CyberStrikeAI | Source in DingTalk Open Platform |
|------------------------|----------------------------------|
| Enable DingTalk robot | Check to enable |
| Client ID (AppKey) | Credentials and basic info → **Client ID (formerly AppKey)** |
| Client Secret | Credentials and basic info → **Client Secret (formerly AppSecret)** |

---

### 3.2 Lark (Feishu)

| Field | Description |
|-------|-------------|
| Enable Lark robot | Check to start the Lark long-lived connection |
| App ID | From Lark open platform app credentials |
| App Secret | From Lark open platform app credentials |
| Verify Token | Optional; for event subscription |

**Lark setup in short**: Log in to [Lark Open Platform](https://open.feishu.cn) → Create an enterprise app → In “Credentials and basic info” get **App ID** and **App Secret** → In “Application capabilities” enable **Robot** and the right permissions → Publish the app → Enter App ID and App Secret in CyberStrikeAI robot settings → Save and **restart** the app.

---

### 3.3 Telegram

| Field | Description |
|-------|-------------|
| Enable Telegram bot | Check to start the Telegram long-polling connection |
| Bot Token | Token issued by @BotFather when you create a new bot |
| Allowed User IDs | Comma-separated Telegram user IDs that may use the bot; leave empty to allow everyone |

**How Telegram bots work in CyberStrikeAI**

CyberStrikeAI uses Telegram's **long-polling** mechanism (`getUpdates`): the app continuously polls Telegram's Bot API servers for new messages. No public IP address, no webhook URL, and no port-forwarding are required—the connection is outbound-only.

**Telegram setup (step-by-step)**

1. **Create a bot via @BotFather**
   - Open Telegram and search for **@BotFather** (official, verified account).
   - Send `/newbot` and follow the prompts (choose a name and username for your bot).
   - BotFather will reply with a **token** that looks like `123456789:ABCDEFGHIJabcdefghij...`.
   - Copy the full token; you will need it in the next step.

2. **Enter the token in CyberStrikeAI**
   - Log in to the CyberStrikeAI web UI.
   - Go to **System Settings** → **Bot Settings** → **Telegram**.
   - Enable the toggle and paste the bot token in the **Bot Token** field.
   - Click **Apply configuration** and **restart CyberStrikeAI**.

3. **Restrict access (recommended)**
   - By default, *any* Telegram user who finds your bot can send messages to it.
   - To restrict access, enter a comma-separated list of your Telegram user IDs in **Allowed User IDs**.
   - Find your Telegram user ID by messaging [@userinfobot](https://t.me/userinfobot) on Telegram.
   - Example: `123456789,987654321` — only those two users may use the bot.
   - Leave the field empty to allow all users (not recommended for production).

4. **Start chatting**
   - In Telegram, search for your bot by its username and open a **direct chat**.
   - Send `help` to see the available commands.
   - In groups, the bot only responds to messages that **@ mention** it (e.g. `@YourBot scan this target`).

**Field mapping (Telegram)**

| Field in CyberStrikeAI | Source |
|------------------------|--------|
| Enable Telegram bot | Check to enable long-polling |
| Bot Token | @BotFather → `/newbot` → copy the token |
| Allowed User IDs | Your Telegram user ID from @userinfobot |

**Progress streaming**

Unlike DingTalk and Lark (which return a single reply at the end), the Telegram bot provides **live progress updates**:

1. When you send a message, the bot immediately replies with `⏳ Processing your request...`.
2. As the AI agent works (calling tools, running scans, analyzing results), the message is periodically **edited in place** to show the current step — e.g. `⚙️ Working... (step 3) calling tool: nmap`.
3. A “typing...” indicator is shown while the agent is thinking.
4. When the agent finishes, the message is updated with the **complete final response**.
5. Responses longer than Telegram's 4096-character limit are split into multiple messages automatically.

**Multiple user support**

Each Telegram user gets an independent conversation session. Session state (active conversation, selected role) is tracked per user ID, exactly like DingTalk and Lark. Multiple users can interact with the bot simultaneously without interfering with each other.

---

## 4. Bot commands

Send these **text commands** to the bot in DingTalk, Lark, or Telegram (text only):

| Command | Description |
|---------|-------------|
| **help** | Show command help |
| **list** or **conversations** | List all conversation titles and IDs |
| **switch \<conversationID\>** or **continue \<conversationID\>** | Continue in the given conversation |
| **new** | Start a new conversation |
| **clear** | Clear current context (same effect as new conversation) |
| **current** | Show current conversation ID and title |
| **stop** | Abort the currently running task |
| **roles** or **role list** | List all available roles (penetration testing, CTF, Web scan, etc.) |
| **role \<roleName\>** or **switch role \<roleName\>** | Switch to the specified role |
| **delete \<conversationID\>** | Delete the specified conversation |
| **version** | Show current CyberStrikeAI version |

Any other text is sent to the AI as a user message, same as in the web UI (e.g. penetration testing, security analysis).

---

## 5. How to use (do I need to @ the bot?)

- **Direct chat (recommended)**: In DingTalk, Lark, or Telegram, **search for the bot and open a direct chat**. Type “help” or any message; **no @ needed**.
- **Group chat**: If the bot is in a group, only messages that **@ the bot** are received and answered; other group messages are ignored.

Summary: **Direct chat** — just send; **in a group** — @ the bot first, then send.

---

## 6. Recommended flow (so you don’t skip steps)

1. **In the open platform**: Complete app creation, copy credentials, enable the bot (DingTalk: **Stream mode**), set permissions, and publish (Section 3).  
2. **In CyberStrikeAI**: System settings → Robot settings → Enable the platform, paste Client ID/App ID and Client Secret/App Secret → **Apply configuration**.  
3. **Restart the CyberStrikeAI process** (otherwise the long-lived connection is not established).  
4. **On your phone**: Open DingTalk or Lark, find the bot (direct chat or @ in a group), send “help” or any message to test.

If the bot does not respond, see **Section 9 (troubleshooting)** and **Section 10 (common pitfalls)**.

---

## 7. Config file example

Example `robots` section in `config.yaml`:

```yaml
robots:
  dingtalk:
    enabled: true
    client_id: "your_dingtalk_app_key"
    client_secret: "your_dingtalk_app_secret"
  lark:
    enabled: true
    app_id: "your_lark_app_id"
    app_secret: "your_lark_app_secret"
    verify_token: ""
  telegram:
    enabled: true
    bot_token: "123456789:ABCDEFGHIJabcdefghij..."
    allowed_user_ids:          # Leave empty to allow all users
      - 123456789
      - 987654321
```

**Restart the app** after changes; the long-lived connection is created at startup.

---

## 8. Testing without DingTalk/Lark/Telegram installed

You can verify bot logic with the **test API** (no messenger client needed):

1. Log in to the CyberStrikeAI web UI (so you have a session).
2. Call the test endpoint with curl (include your session Cookie):

```bash
# Replace YOUR_COOKIE with the Cookie from your browser (F12 → Network → any request → Request headers → Cookie)
curl -X POST "http://localhost:8080/api/robot/test" \
  -H "Content-Type: application/json" \
  -H "Cookie: YOUR_COOKIE" \
  -d '{"platform":"telegram","user_id":"123456789","text":"help"}'
```

If the JSON response contains `"reply":"[CyberStrikeAI Bot Commands]..."`, command handling works. You can also try `"text":"list"` or `"text":"current"`.

API: `POST /api/robot/test` (requires login). Body: `{"platform":"optional","user_id":"optional","text":"required"}`. Response: `{"reply":"..."}`.

---

## 9. Telegram: no response when sending messages

Check in this order:

1. **Bot token is correct**
   Check the `robots.telegram.bot_token` field in `config.yaml` — paste it directly from @BotFather without extra spaces. If the token is wrong, you will see `Telegram API error: Unauthorized` in the application logs.

2. **Did you restart after saving?**
   Telegram long-polling starts at **startup**. "Apply configuration" only updates the config file; **restart CyberStrikeAI** for the connection to take effect.

3. **Application logs**
   - On startup you should see: `Telegram bot connecting...` followed by `Telegram bot started username=@YourBot`.
   - After sending a message, you should see `Telegram message received user_id=... content_preview=...` in the logs.
   - `getMe failed: Unauthorized` → wrong token; re-copy it from @BotFather.
   - `Telegram bot polling error, will reconnect` → network error; the bot will retry automatically.

4. **User is not in the whitelist**
   If `allowed_user_ids` is set and your user ID is not in the list, the bot replies `⛔ You are not authorized to use this bot.` — add your ID to the whitelist or leave the list empty.

5. **Group chat: forgot to @ the bot**
   In group chats, the bot ignores messages that do not mention it. Send `@YourBot help` to test.

6. **Network connectivity**
   The server running CyberStrikeAI must be able to reach `api.telegram.org` (port 443). If the server is behind a firewall or in a region where Telegram is blocked, set up a proxy or use a VPN.

---

## 10. DingTalk: no response when sending messages

Check in this order:

0. **After laptop sleep or network drop**  
   DingTalk and Lark both use long-lived connections; they break when the machine sleeps or the network drops. The app **auto-reconnects** (retries within about 5–60 seconds). After wake or network recovery, wait a moment before sending; if there is still no response, restart the CyberStrikeAI process.

1. **Client ID / Client Secret match the open platform exactly**  
   Copy from “Credentials and basic info”; avoid typing. Watch **0** vs **o** and **1** vs **l** (e.g. `ding9gf9tiozuc504aer` has **504**, not 5o4).

2. **Did you restart after saving?**  
   The long-lived connection is created at **startup**. “Apply configuration” only updates the config file; you **must restart the CyberStrikeAI process** for the DingTalk connection to start.

3. **Application logs**
   - On startup you should see: `DingTalk Stream connecting...`, `DingTalk Stream started (no public IP required), waiting for messages`.
   - If you see `DingTalk Stream long-lived connection exited` with an error, it’s usually wrong **Client ID / Client Secret** or **Stream not enabled** in the open platform.
   - After sending a message in DingTalk, you should see `DingTalk message received` in the logs; if not, the platform is not pushing to this app (check that the bot is enabled and **Stream mode** is selected).

4. **Open platform**  
   The app must be **published**. Under “Robot” you must enable **Stream** for receiving messages (HTTP callback only is not enough). Permission management must include robot receive/send message permissions.

---

## 11. Common pitfalls

- **Wrong bot type**: The “Custom” bot added in a DingTalk **group** (Webhook + sign secret) **cannot** be used for two-way chat. Only the **enterprise internal app** bot from the open platform is supported.  
- **Saved but not restarted**: After changing robot settings in CyberStrikeAI you **must restart** the app, or the long-lived connection will not be established.  
- **Client ID typo**: If the platform shows `504`, use `504` (not `5o4`); prefer copy/paste.  
- **DingTalk: only HTTP callback, no Stream**: This app receives messages via **Stream**. In the open platform, message reception must be **Stream mode**.  
- **App not published**: After changing the bot or permissions in the open platform, **publish a new version** under “Version management and release”, or changes won’t apply.

---

## 12. Common pitfalls (Telegram-specific)

- **Wrong bot type**: Only bots created with **@BotFather** are supported. You cannot use a regular Telegram user account as a bot.
- **Token leakage**: The bot token grants full control over the bot. Never commit it to version control; use `config.yaml` or environment variables, and restrict access with `allowed_user_ids`.
- **No public IP needed**: Unlike some webhook-based bots, CyberStrikeAI uses long-polling. No port-forwarding or public IP is required — any machine with outbound HTTPS access to `api.telegram.org` works.
- **Rate limits**: Telegram allows up to 30 messages/second globally and 1 message/second per chat. Progress edits are throttled internally (at most once every 3 seconds) to stay within limits.
- **Message length**: Responses longer than 4096 characters are automatically split into multiple messages.

---

## 13. Notes

- All platforms: **text messages only**; other types (e.g. image, voice, sticker) are not supported and are ignored.
- Conversations are shared with the web UI: conversations created from any bot appear in the web “Conversations” list and vice versa.
- Bot execution uses the same logic as **`/api/agent-loop/stream`** (progress callbacks, process details stored in the DB).
- DingTalk and Lark return a single reply at the end of execution. Telegram additionally shows **live progress updates** by editing the placeholder message during execution.
