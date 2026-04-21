#!/usr/bin/env python3
"""
CyberStrikeAI - DroidRun LLM Proxy Service
============================================
Acts as a bridge between CyberStrikeAI's MCP tools and DroidRun's device
control layer. Provides LLM-friendly formatted device state (UI tree +
screenshots) and translates high-level LLM commands to device actions.

This is easier for the LLM than raw ADB commands because:
1. UI elements are indexed - LLM says "click(3)" instead of pixel coordinates
2. State is pre-formatted as readable text the LLM can reason about
3. Screenshots are returned as base64 for vision-language models (Qwen3.5 VL)
4. Actions return structured success/failure with descriptions

Architecture:
    LLM (Qwen3.5 VL) → CyberStrikeAI MCP → This Proxy → DroidRun → Cuttlefish VM

Usage:
    # Start as HTTP service (default port 18090)
    python3 droidrun_proxy.py --port 18090

    # Or import and use programmatically
    from droidrun_proxy import DroidRunProxy
    proxy = DroidRunProxy()
    await proxy.connect()
    state = await proxy.get_state()
"""

import asyncio
import base64
import json
import logging
import os
import sys
import time
import traceback
from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Any, Optional

# Add droidrun to path
DROIDRUN_PATH = os.environ.get("DROIDRUN_PATH", str(Path.home() / "droidrun"))
if os.path.isdir(DROIDRUN_PATH):
    sys.path.insert(0, DROIDRUN_PATH)

logger = logging.getLogger("droidrun_proxy")

# ─── Data Structures ────────────────────────────────────────────────────────

@dataclass
class DeviceState:
    """LLM-friendly device state representation."""
    connected: bool = False
    ui_text: str = ""            # Formatted UI tree (indexed elements)
    focused_element: str = ""    # Currently focused element text
    current_app: str = ""        # Current foreground app package
    current_activity: str = ""   # Current activity name
    screen_width: int = 0
    screen_height: int = 0
    screenshot_b64: str = ""     # Base64-encoded PNG screenshot
    element_count: int = 0       # Number of interactive elements
    keyboard_visible: bool = False
    timestamp: float = 0.0

    def to_llm_text(self, include_screenshot: bool = False) -> str:
        """Format state as text for LLM consumption."""
        parts = [
            f"=== Android Device State (t={int(self.timestamp)}) ===",
            f"App: {self.current_app}",
            f"Activity: {self.current_activity}",
            f"Screen: {self.screen_width}x{self.screen_height}",
            f"Elements: {self.element_count}",
            f"Keyboard: {'visible' if self.keyboard_visible else 'hidden'}",
            f"Focused: {self.focused_element or 'none'}",
            "",
            "--- UI Elements (click by index) ---",
            self.ui_text or "(no elements visible)",
        ]
        if include_screenshot and self.screenshot_b64:
            parts.append(f"\n[Screenshot attached as base64 PNG, {len(self.screenshot_b64)} chars]")
        return "\n".join(parts)

    def to_dict(self) -> dict:
        d = asdict(self)
        d["llm_text"] = self.to_llm_text()
        return d


@dataclass
class ActionResult:
    """Result of a device action."""
    success: bool
    action: str
    summary: str
    error: str = ""
    state_after: Optional[DeviceState] = None

    def to_llm_text(self) -> str:
        status = "OK" if self.success else "FAILED"
        parts = [f"[{status}] {self.action}: {self.summary}"]
        if self.error:
            parts.append(f"Error: {self.error}")
        if self.state_after:
            parts.append("")
            parts.append(self.state_after.to_llm_text())
        return "\n".join(parts)

    def to_dict(self) -> dict:
        d = {
            "success": self.success,
            "action": self.action,
            "summary": self.summary,
            "error": self.error,
        }
        if self.state_after:
            d["state_after"] = self.state_after.to_dict()
        return d


# ─── DroidRun Proxy ─────────────────────────────────────────────────────────

class DroidRunProxy:
    """
    Proxy between LLM and Android device via DroidRun.
    Provides high-level, LLM-friendly device interaction.
    """

    def __init__(
        self,
        serial: str | None = None,
        use_tcp: bool = False,
        use_vision: bool = True,
        screenshot_dir: str = "/tmp/droidrun_screenshots",
    ):
        self.serial = serial
        self.use_tcp = use_tcp
        self.use_vision = use_vision
        self.screenshot_dir = screenshot_dir
        self._driver = None
        self._state_provider = None
        self._connected = False
        self._last_state: Optional[DeviceState] = None
        self._screenshot_count = 0
        os.makedirs(screenshot_dir, exist_ok=True)

    async def connect(self, serial: str | None = None) -> bool:
        """Connect to the Android device via DroidRun."""
        try:
            from droidrun.tools.driver.android import AndroidDriver
            from droidrun.tools.ui.provider import AndroidStateProvider
            from droidrun.tools.filters.concise_filter import ConciseFilter
            from droidrun.tools.formatters.indexed_formatter import IndexedFormatter

            s = serial or self.serial
            self._driver = AndroidDriver(serial=s, use_tcp=self.use_tcp)
            await self._driver.connect()

            tree_filter = ConciseFilter()
            tree_formatter = IndexedFormatter()
            self._state_provider = AndroidStateProvider(
                driver=self._driver,
                tree_filter=tree_filter,
                tree_formatter=tree_formatter,
            )
            self._connected = True
            logger.info(f"Connected to device: {s or 'auto-detect'}")
            return True
        except Exception as e:
            logger.error(f"Connection failed: {e}")
            self._connected = False
            return False

    async def disconnect(self):
        """Disconnect from the device."""
        self._driver = None
        self._state_provider = None
        self._connected = False

    @property
    def connected(self) -> bool:
        return self._connected and self._driver is not None

    # ─── State Observation ───────────────────────────────────────────────

    async def get_state(self, include_screenshot: bool = True) -> DeviceState:
        """
        Get current device state in LLM-friendly format.
        Returns indexed UI elements + optional base64 screenshot.
        """
        if not self.connected:
            return DeviceState(connected=False, ui_text="[Not connected to device]")

        try:
            ui_state = await self._state_provider.get_state()

            screenshot_b64 = ""
            if include_screenshot and self.use_vision:
                screenshot_b64 = await self._take_screenshot_b64()

            phone = ui_state.phone_state or {}
            state = DeviceState(
                connected=True,
                ui_text=ui_state.formatted_text,
                focused_element=ui_state.focused_text or "",
                current_app=phone.get("packageName", phone.get("currentApp", "")),
                current_activity=phone.get("currentActivity", ""),
                screen_width=ui_state.screen_width,
                screen_height=ui_state.screen_height,
                screenshot_b64=screenshot_b64,
                element_count=len(ui_state.elements) if ui_state.elements else 0,
                keyboard_visible=phone.get("isKeyboardVisible", False),
                timestamp=time.time(),
            )
            self._last_state = state
            return state
        except Exception as e:
            logger.error(f"get_state failed: {e}")
            return DeviceState(connected=True, ui_text=f"[Error getting state: {e}]")

    async def _take_screenshot_b64(self) -> str:
        """Take a screenshot and return as base64 PNG."""
        try:
            png_bytes = await self._driver.screenshot()
            if png_bytes:
                # Save to disk for debugging/file manager
                self._screenshot_count += 1
                path = os.path.join(
                    self.screenshot_dir,
                    f"screen_{self._screenshot_count:04d}.png"
                )
                with open(path, "wb") as f:
                    f.write(png_bytes)
                return base64.b64encode(png_bytes).decode("ascii")
        except Exception as e:
            logger.warning(f"Screenshot failed: {e}")
        return ""

    async def get_screenshot(self) -> tuple[str, str]:
        """Take screenshot, return (base64_png, file_path)."""
        if not self.connected:
            return "", ""
        try:
            png_bytes = await self._driver.screenshot()
            self._screenshot_count += 1
            path = os.path.join(
                self.screenshot_dir,
                f"screen_{self._screenshot_count:04d}.png"
            )
            with open(path, "wb") as f:
                f.write(png_bytes)
            b64 = base64.b64encode(png_bytes).decode("ascii")
            return b64, path
        except Exception as e:
            return "", ""

    # ─── Actions ─────────────────────────────────────────────────────────

    async def _exec_action(self, action_name: str, fn, return_state: bool = True) -> ActionResult:
        """Execute an action and return result with optional state update."""
        try:
            result = await fn()
            summary = result if isinstance(result, str) else str(result)

            state_after = None
            if return_state:
                await asyncio.sleep(0.5)  # Wait for UI to settle
                state_after = await self.get_state(include_screenshot=self.use_vision)

            return ActionResult(
                success=True,
                action=action_name,
                summary=summary,
                state_after=state_after,
            )
        except Exception as e:
            return ActionResult(
                success=False,
                action=action_name,
                summary="",
                error=str(e),
            )

    async def click(self, index: int) -> ActionResult:
        """Click a UI element by its index number."""
        if not self.connected:
            return ActionResult(False, "click", "", "Not connected")

        ui_state = await self._state_provider.get_state()
        coords = ui_state.get_element_coords(index)
        if coords is None:
            return ActionResult(False, f"click({index})", "", f"Element {index} not found")

        x, y = coords
        info = ui_state.get_element_info(index) or {}
        el_text = info.get("text", "") or info.get("className", "")

        return await self._exec_action(
            f"click({index}) [{el_text}]",
            lambda: self._driver.tap(x, y)
        )

    async def long_press(self, index: int) -> ActionResult:
        """Long press a UI element by index."""
        if not self.connected:
            return ActionResult(False, "long_press", "", "Not connected")

        ui_state = await self._state_provider.get_state()
        coords = ui_state.get_element_coords(index)
        if coords is None:
            return ActionResult(False, f"long_press({index})", "", f"Element {index} not found")

        x, y = coords
        # Long press = tap and hold via swipe to same point with duration
        return await self._exec_action(
            f"long_press({index})",
            lambda: self._driver.swipe(x, y, x, y, duration_ms=1500)
        )

    async def tap_at(self, x: int, y: int) -> ActionResult:
        """Tap at specific screen coordinates."""
        if not self.connected:
            return ActionResult(False, "tap_at", "", "Not connected")
        return await self._exec_action(
            f"tap_at({x}, {y})",
            lambda: self._driver.tap(x, y)
        )

    async def type_text(self, text: str, index: int = -1, clear: bool = False) -> ActionResult:
        """Type text. If index >= 0, click the element first."""
        if not self.connected:
            return ActionResult(False, "type_text", "", "Not connected")

        if index >= 0:
            click_result = await self.click(index)
            if not click_result.success:
                return click_result
            await asyncio.sleep(0.3)

        return await self._exec_action(
            f"type_text('{text[:30]}...', index={index})" if len(text) > 30
            else f"type_text('{text}', index={index})",
            lambda: self._driver.input_text(text, clear=clear)
        )

    async def swipe(self, x1: int, y1: int, x2: int, y2: int, duration_ms: int = 500) -> ActionResult:
        """Swipe gesture between two points."""
        if not self.connected:
            return ActionResult(False, "swipe", "", "Not connected")
        return await self._exec_action(
            f"swipe({x1},{y1} → {x2},{y2})",
            lambda: self._driver.swipe(x1, y1, x2, y2, duration_ms=duration_ms)
        )

    async def scroll_down(self) -> ActionResult:
        """Scroll down on the current screen."""
        if not self.connected:
            return ActionResult(False, "scroll_down", "", "Not connected")
        ui = await self._state_provider.get_state()
        w, h = ui.screen_width, ui.screen_height
        return await self.swipe(w // 2, h * 3 // 4, w // 2, h // 4, 500)

    async def scroll_up(self) -> ActionResult:
        """Scroll up on the current screen."""
        if not self.connected:
            return ActionResult(False, "scroll_up", "", "Not connected")
        ui = await self._state_provider.get_state()
        w, h = ui.screen_width, ui.screen_height
        return await self.swipe(w // 2, h // 4, w // 2, h * 3 // 4, 500)

    async def press_back(self) -> ActionResult:
        """Press the Android Back button."""
        if not self.connected:
            return ActionResult(False, "press_back", "", "Not connected")
        return await self._exec_action(
            "press_back",
            lambda: self._driver.press_key(4)  # KEYCODE_BACK
        )

    async def press_home(self) -> ActionResult:
        """Press the Android Home button."""
        if not self.connected:
            return ActionResult(False, "press_home", "", "Not connected")
        return await self._exec_action(
            "press_home",
            lambda: self._driver.press_key(3)  # KEYCODE_HOME
        )

    async def press_enter(self) -> ActionResult:
        """Press Enter/Return key."""
        if not self.connected:
            return ActionResult(False, "press_enter", "", "Not connected")
        return await self._exec_action(
            "press_enter",
            lambda: self._driver.press_key(66)  # KEYCODE_ENTER
        )

    async def open_app(self, package_name: str) -> ActionResult:
        """Open an app by package name."""
        if not self.connected:
            return ActionResult(False, "open_app", "", "Not connected")
        return await self._exec_action(
            f"open_app({package_name})",
            lambda: self._driver.start_app(package_name)
        )

    async def install_apk(self, apk_path: str) -> ActionResult:
        """Install an APK file."""
        if not self.connected:
            return ActionResult(False, "install_apk", "", "Not connected")
        return await self._exec_action(
            f"install_apk({os.path.basename(apk_path)})",
            lambda: self._driver.install_app(apk_path, reinstall=True),
            return_state=False,
        )

    async def list_apps(self, include_system: bool = False) -> ActionResult:
        """List installed apps."""
        if not self.connected:
            return ActionResult(False, "list_apps", "", "Not connected")
        try:
            apps = await self._driver.get_apps(include_system=include_system)
            app_lines = [f"  {a.get('package', '')} - {a.get('label', '')}" for a in apps]
            summary = f"Found {len(apps)} apps:\n" + "\n".join(app_lines)
            return ActionResult(True, "list_apps", summary)
        except Exception as e:
            return ActionResult(False, "list_apps", "", str(e))

    async def wait(self, seconds: float = 1.0) -> ActionResult:
        """Wait for a specified duration."""
        await asyncio.sleep(seconds)
        state = await self.get_state(include_screenshot=self.use_vision)
        return ActionResult(True, f"wait({seconds}s)", f"Waited {seconds}s", state_after=state)


# ─── HTTP Service ────────────────────────────────────────────────────────────

async def run_http_service(port: int = 18090, serial: str | None = None):
    """Run the proxy as an HTTP API service."""
    try:
        from aiohttp import web
    except ImportError:
        logger.error("aiohttp required: pip install aiohttp")
        return

    proxy = DroidRunProxy(serial=serial)
    connected = await proxy.connect()
    if not connected:
        logger.error("Failed to connect to device")
        return

    routes = web.RouteTableDef()

    @routes.get("/status")
    async def status(request):
        return web.json_response({"connected": proxy.connected})

    @routes.get("/state")
    async def state(request):
        include_screenshot = request.query.get("screenshot", "true").lower() == "true"
        s = await proxy.get_state(include_screenshot=include_screenshot)
        return web.json_response(s.to_dict())

    @routes.get("/state/text")
    async def state_text(request):
        s = await proxy.get_state(include_screenshot=False)
        return web.Response(text=s.to_llm_text())

    @routes.get("/screenshot")
    async def screenshot(request):
        b64, path = await proxy.get_screenshot()
        return web.json_response({"base64": b64, "path": path})

    @routes.post("/action")
    async def action(request):
        data = await request.json()
        action_name = data.get("action", "")
        params = data.get("params", {})
        method = getattr(proxy, action_name, None)
        if method is None:
            return web.json_response(
                {"success": False, "error": f"Unknown action: {action_name}"},
                status=400,
            )
        result = await method(**params)
        return web.json_response(result.to_dict())

    # Convenience endpoints
    @routes.post("/click")
    async def click(request):
        data = await request.json()
        r = await proxy.click(data["index"])
        return web.json_response(r.to_dict())

    @routes.post("/type")
    async def type_text(request):
        data = await request.json()
        r = await proxy.type_text(data["text"], data.get("index", -1), data.get("clear", False))
        return web.json_response(r.to_dict())

    @routes.post("/swipe")
    async def swipe(request):
        data = await request.json()
        r = await proxy.swipe(data["x1"], data["y1"], data["x2"], data["y2"], data.get("duration_ms", 500))
        return web.json_response(r.to_dict())

    app = web.Application()
    app.add_routes(routes)
    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, "0.0.0.0", port)
    logger.info(f"DroidRun proxy service starting on port {port}")
    await site.start()
    await asyncio.Event().wait()  # Run forever


if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description="DroidRun LLM Proxy Service")
    parser.add_argument("--port", type=int, default=18090)
    parser.add_argument("--serial", type=str, default=None)
    parser.add_argument("--no-vision", action="store_true")
    args = parser.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(message)s")

    asyncio.run(run_http_service(port=args.port, serial=args.serial))
