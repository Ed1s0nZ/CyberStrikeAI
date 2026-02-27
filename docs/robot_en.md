# CyberStrikeAI Robot / Chatbot Guide

[中文](robot.md)

This document explains how to chat with CyberStrikeAI from **DingTalk** and **Lark (Feishu)** using long-lived connections—no need to open a browser on the server. Following the steps below helps avoid common mistakes.

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
   - **Critical**: set message reception to **“Stream mode”** (流式接入). If you only enable “HTTP callback” or do not select Stream, CyberStrikeAI will not receive messages.  
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

## 4. Bot commands

Send these **text commands** to the bot in DingTalk or Lark (text only):

| Command | Description |
|---------|-------------|
| **帮助** (help) | Show command help |
| **列表** or **对话列表** (list) | List all conversation titles and IDs |
| **切换 \<conversationID\>** or **继续 \<conversationID\>** | Continue in the given conversation |
| **新对话** (new) | Start a new conversation |
| **清空** (clear) | Clear current context (same effect as new conversation) |
| **当前** (current) | Show current conversation ID and title |

Any other text is sent to the AI as a user message, same as in the web UI (e.g. penetration testing, security analysis).

---

## 5. How to use (do I need to @ the bot?)

- **Direct chat (recommended)**: In DingTalk or Lark, **search for the bot and open a direct chat**. Type “帮助” or any message; **no @ needed**.  
- **Group chat**: If the bot is in a group, only messages that **@ the bot** are received and answered; other group messages are ignored.

Summary: **Direct chat** — just send; **in a group** — @ the bot first, then send.

---

## 6. Recommended flow (so you don’t skip steps)

1. **In the open platform**: Complete app creation, copy credentials, enable the bot (DingTalk: **Stream mode**), set permissions, and publish (Section 3).  
2. **In CyberStrikeAI**: System settings → Robot settings → Enable the platform, paste Client ID/App ID and Client Secret/App Secret → **Apply configuration**.  
3. **Restart the CyberStrikeAI process** (otherwise the long-lived connection is not established).  
4. **On your phone**: Open DingTalk or Lark, find the bot (direct chat or @ in a group), send “帮助” or any message to test.

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
```

**Restart the app** after changes; the long-lived connection is created at startup.

---

## 8. Testing without DingTalk/Lark installed

You can verify bot logic with the **test API** (no DingTalk/Lark client needed):

1. Log in to the CyberStrikeAI web UI (so you have a session).
2. Call the test endpoint with curl (include your session Cookie):

```bash
# Replace YOUR_COOKIE with the Cookie from your browser (F12 → Network → any request → Request headers → Cookie)
curl -X POST "http://localhost:8080/api/robot/test" \
  -H "Content-Type: application/json" \
  -H "Cookie: YOUR_COOKIE" \
  -d '{"platform":"dingtalk","user_id":"test_user","text":"帮助"}'
```

If the JSON response contains `"reply":"【CyberStrikeAI 机器人命令】..."`, command handling works. You can also try `"text":"列表"` or `"text":"当前"`.

API: `POST /api/robot/test` (requires login). Body: `{"platform":"optional","user_id":"optional","text":"required"}`. Response: `{"reply":"..."}`.

---

## 9. DingTalk: no response when sending messages

Check in this order:

1. **Client ID / Client Secret match the open platform exactly**  
   Copy from “Credentials and basic info”; avoid typing. Watch **0** vs **o** and **1** vs **l** (e.g. `ding9gf9tiozuc504aer` has **504**, not 5o4).

2. **Did you restart after saving?**  
   The long-lived connection is created at **startup**. “Apply configuration” only updates the config file; you **must restart the CyberStrikeAI process** for the DingTalk connection to start.

3. **Application logs**  
   - On startup you should see: `钉钉 Stream 正在连接…`, `钉钉 Stream 已启动（无需公网），等待收消息`.  
   - If you see `钉钉 Stream 长连接退出` with an error, it’s usually wrong **Client ID / Client Secret** or **Stream not enabled** in the open platform.  
   - After sending a message in DingTalk, you should see `钉钉收到消息` in the logs; if not, the platform is not pushing to this app (check that the bot is enabled and **Stream mode** is selected).

4. **Open platform**  
   The app must be **published**. Under “Robot” you must enable **Stream** for receiving messages (HTTP callback only is not enough). Permission management must include robot receive/send message permissions.

---

## 10. Common pitfalls

- **Wrong bot type**: The “Custom” bot added in a DingTalk **group** (Webhook + sign secret) **cannot** be used for two-way chat. Only the **enterprise internal app** bot from the open platform is supported.  
- **Saved but not restarted**: After changing robot settings in CyberStrikeAI you **must restart** the app, or the long-lived connection will not be established.  
- **Client ID typo**: If the platform shows `504`, use `504` (not `5o4`); prefer copy/paste.  
- **DingTalk: only HTTP callback, no Stream**: This app receives messages via **Stream**. In the open platform, message reception must be **Stream mode**.  
- **App not published**: After changing the bot or permissions in the open platform, **publish a new version** under “Version management and release”, or changes won’t apply.

---

## 11. Notes

- DingTalk and Lark: **text messages only**; other types (e.g. image, voice) are not supported and may be ignored.  
- Conversations are shared with the web UI: conversations created from the bot appear in the web “Conversations” list and vice versa.  
- Bot execution uses the same logic as **`/api/agent-loop/stream`** (progress callbacks, process details stored in the DB); only the final reply is sent back to DingTalk/Lark in one message (no SSE to the client).
