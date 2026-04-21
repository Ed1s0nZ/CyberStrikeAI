# DroidRun Proxy - LLM-Friendly Android Device Control

## Overview
- Tool category: DroidRun proxy tools (12 MCP tools)
- Proxy service: `droidrun_proxy.py` (HTTP on port 18090)
- Purpose: High-level, LLM-friendly Android device interaction via DroidRun
- Vision support: Screenshots as base64 PNG for Qwen3.5 VL analysis
- Prerequisite: Cuttlefish VM running + DroidRun Portal APK installed

## Why Use DroidRun Proxy Instead of Raw ADB

| Feature | Raw ADB (cuttlefish_shell) | DroidRun Proxy (droidrun_*) |
|---------|---------------------------|----------------------------|
| Element targeting | Pixel coordinates (guess) | Indexed elements (click by number) |
| Screen understanding | Parse XML accessibility dump | Pre-formatted readable text |
| Vision | Manual screencap + file read | Base64 PNG in every state response |
| Action feedback | Exit code only | Success/failure + updated state |
| Typing | `input text "..."` (slow) | Portal IME (fast, reliable) |
| Scrolling | Calculate coordinates manually | `droidrun_scroll up/down` |
| Learning curve | Know ADB commands | Just read state → pick index → act |

## Available Tools

### droidrun_connect
Check if the DroidRun proxy service is running and connected to the device.
```
Call first before using any other droidrun_* tool.
If not running, tell user: python3 ~/CyberStrikeAI/scripts/cuttlefish/droidrun_proxy.py
```

### droidrun_state
**The most important tool.** Returns the complete device state:
- Current app and activity
- All visible UI elements with index numbers
- Screen dimensions, keyboard state, focused element
- Optional base64 PNG screenshot for visual analysis

**Example output:**
```
=== Android Device State (t=1709991234) ===
App: com.android.settings
Activity: .Settings
Screen: 1080x2400
Elements: 12
Keyboard: hidden
Focused: none

--- UI Elements (click by index) ---
[0] FrameLayout '' (0,0,1080,2400)
  [1] TextView 'Settings' (40,80,200,120)
  [2] RecyclerView '' (0,160,1080,2400)
    [3] LinearLayout '' (0,160,1080,240)
      [4] TextView 'Network & internet' (80,170,600,210)
      [5] TextView 'Wi-Fi, mobile, data usage' (80,210,600,240)
    [6] LinearLayout '' (0,240,1080,320)
      [7] TextView 'Connected devices' (80,250,600,290)
      [8] TextView 'Bluetooth, NFC' (80,290,600,320)
    [9] LinearLayout '' (0,320,1080,400)
      [10] TextView 'Apps' (80,330,600,370)
      [11] TextView 'Permissions, default apps' (80,370,600,400)
```

### droidrun_click
Click a UI element by its index number.
```
Example: droidrun_state shows "[4] TextView 'Network & internet'"
Action: droidrun_click index=4
Result: [OK] click(4) [Network & internet]: None
        === Android Device State === (shows the new screen)
```

### droidrun_type
Type text into a UI element. Clicks the element first if index provided.
```
Example: Type username into a login field
  droidrun_type text="admin@example.com" index=5 clear=true

Example: Type into already-focused field
  droidrun_type text="password123"
```

### droidrun_scroll
Scroll the screen up or down.
```
droidrun_scroll direction="down"  - scroll down to see more content
droidrun_scroll direction="up"    - scroll back up
```

### droidrun_button
Press system buttons.
```
droidrun_button button="back"   - navigate back
droidrun_button button="home"   - go to home screen
droidrun_button button="enter"  - submit text input / confirm
```

### droidrun_open_app
Launch an app by package name.
```
droidrun_open_app package_name="com.android.settings"
droidrun_open_app package_name="com.target.app"
```

### droidrun_list_apps
List installed applications.
```
droidrun_list_apps                        - user-installed apps only
droidrun_list_apps include_system=true    - all apps including system
```

### droidrun_install
Install an APK file.
```
droidrun_install apk_path="/tmp/target_app.apk"
```

### droidrun_screenshot
Take a screenshot (base64 PNG). Use droidrun_state instead if you also need element indices.
```
Returns: base64 PNG string + saved file path
```

### droidrun_swipe
Custom swipe gesture between coordinates.
```
droidrun_swipe x1=540 y1=1800 x2=540 y2=600 duration_ms=500
```

### droidrun_wait
Wait for a duration, then return fresh state.
```
droidrun_wait seconds=2.0  - wait for loading/animation to complete
```

## Model Usage Guidance

### Standard Interaction Loop
```
1. droidrun_connect          → verify proxy is running
2. droidrun_state            → see what's on screen (read the elements!)
3. Analyze the state text    → decide which element to interact with
4. droidrun_click/type/scroll → take action (by element INDEX)
5. Read the returned state   → see what changed
6. Repeat from step 3        → until goal is achieved
```

### Example: Log Into an App
```
Step 1: droidrun_open_app package_name="com.target.app"
Step 2: droidrun_state → see login screen
  Output shows: [3] EditText 'Username' [5] EditText 'Password' [7] Button 'Sign In'
Step 3: droidrun_type text="admin" index=3
Step 4: droidrun_type text="P@ssw0rd" index=5
Step 5: droidrun_click index=7
Step 6: droidrun_wait seconds=2.0 → see if login succeeded
```

### Example: Navigate Settings
```
Step 1: droidrun_open_app package_name="com.android.settings"
Step 2: droidrun_state → see settings menu
Step 3: droidrun_click index=4 → click "Network & internet"
Step 4: droidrun_state → see network settings
Step 5: droidrun_scroll direction="down" → see more options
```

### Example: Combined with SSLStrip
```
Step 1: Start SSLStrip on host (via exec tool)
Step 2: cuttlefish_proxy set <host_ip> 10000
Step 3: droidrun_open_app package_name="com.target.app"
Step 4: droidrun_state → see login screen
Step 5: droidrun_type text="test@test.com" index=3
Step 6: droidrun_type text="password" index=5
Step 7: droidrun_click index=7 → submit login
Step 8: Check SSLStrip log for captured credentials
```

### Vision Model Tips (Qwen3.5 VL)
- droidrun_state with include_screenshot=true gives you both text AND visual data
- The text-based element indices are more reliable than visual pixel estimation
- Use screenshots to verify: does the screen look right? Is there an error dialog?
- Screenshots help with non-standard UI elements that accessibility tree might miss
- When elements are ambiguous in text, the screenshot clarifies (icons, colors, layout)

## Architecture
```
Qwen3.5 VL (LLM)
    ↓ MCP tool call
CyberStrikeAI MCP Server (Go)
    ↓ HTTP request
DroidRun Proxy (Python, port 18090)
    ↓ DroidRun API
AndroidDriver → PortalClient → Portal APK
    ↓ ADB / Content Provider
Cuttlefish VM (AOSP on QEMU/KVM)
```

## Configuration
All settings under `agent.cuttlefish` in config.yaml:
- `proxy_port: 18090` - HTTP port for the proxy service
- `proxy_auto_start: true` - auto-start proxy with VM
- `screenshot_dir: /tmp/droidrun_screenshots` - saved screenshots
- `vision_enabled: true` - include base64 PNG in state responses
- `droidrun_path: ~/droidrun` - DroidRun installation
