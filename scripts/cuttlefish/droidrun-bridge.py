#!/usr/bin/env python3
"""
CyberStrikeAI - DroidRun ↔ Cuttlefish Bridge
=============================================
Connects DroidRun agent framework to the Cuttlefish virtual Android device.
Provides automated mobile app testing, APK analysis, and device control
through natural language commands.

Usage:
    python3 droidrun-bridge.py "Open Settings and check device info"
    python3 droidrun-bridge.py --install /path/to/target.apk "Test the app login flow"
    python3 droidrun-bridge.py --config custom_config.yaml "Navigate to profile page"
"""

import asyncio
import argparse
import os
import sys
import subprocess
import json
from pathlib import Path

# Add droidrun to path
DROIDRUN_PATH = Path.home() / "droidrun"
if DROIDRUN_PATH.exists():
    sys.path.insert(0, str(DROIDRUN_PATH))

CVD_HOME = os.environ.get("CVD_HOME", str(Path.home() / "cuttlefish-workspace"))
CVD_ADB = os.path.join(CVD_HOME, "bin", "adb")
DROIDRUN_CONFIG = os.path.join(CVD_HOME, "droidrun", "config.yaml")


def get_cvd_serial() -> str | None:
    """Get the ADB serial of the running Cuttlefish device."""
    try:
        # Try system adb first, then cvd-bundled adb
        for adb in ["adb", CVD_ADB]:
            result = subprocess.run(
                [adb, "devices"],
                capture_output=True, text=True, timeout=5
            )
            for line in result.stdout.strip().split("\n")[1:]:
                if "\tdevice" in line:
                    serial = line.split("\t")[0]
                    # Cuttlefish devices are typically 0.0.0.0:6520 or similar
                    return serial
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass
    return None


def ensure_device_running() -> str:
    """Ensure Cuttlefish is running, launch if needed. Returns serial."""
    serial = get_cvd_serial()
    if serial:
        print(f"[+] Cuttlefish device found: {serial}")
        return serial

    print("[+] No Cuttlefish device detected. Launching...")
    launch_script = os.path.join(CVD_HOME, "cvd-launch.sh")
    if not os.path.exists(launch_script):
        print("[-] Cuttlefish not set up. Run setup.sh first.")
        sys.exit(1)

    subprocess.run([launch_script], check=True)
    serial = get_cvd_serial()
    if not serial:
        print("[-] Failed to detect device after launch.")
        sys.exit(1)

    return serial


def install_apk(apk_path: str, serial: str) -> str:
    """Install APK on device, return package name."""
    print(f"[+] Installing: {apk_path}")
    adb = "adb"
    subprocess.run([adb, "-s", serial, "install", "-r", "-t", apk_path], check=True)

    # Try to get package name
    try:
        result = subprocess.run(
            ["aapt", "dump", "badging", apk_path],
            capture_output=True, text=True, timeout=10
        )
        for line in result.stdout.split("\n"):
            if line.startswith("package: name="):
                pkg = line.split("'")[1]
                print(f"[+] Package: {pkg}")
                return pkg
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass
    return ""


def install_portal(serial: str):
    """Ensure DroidRun Portal APK is installed on the device."""
    try:
        from droidrun.portal import setup_portal
        print("[+] Setting up DroidRun Portal APK...")
        asyncio.run(setup_portal(serial=serial))
    except ImportError:
        print("[!] DroidRun not installed. Install with: pip install droidrun")
        print("[!] Skipping Portal APK setup.")


async def run_droidrun(goal: str, serial: str, config_path: str | None = None):
    """Run DroidRun agent with the given goal on the Cuttlefish device."""
    try:
        from droidrun.agent.droid.droid_agent import DroidAgent
        from droidrun.config_manager.config_manager import DroidrunConfig
        from droidrun.config_manager.loader import ConfigLoader
    except ImportError:
        print("[-] DroidRun not installed. Falling back to CLI mode.")
        _run_droidrun_cli(goal, serial, config_path)
        return

    # Load config
    cfg_path = config_path or DROIDRUN_CONFIG
    if os.path.exists(cfg_path):
        config = ConfigLoader.load(cfg_path)
    else:
        config = DroidrunConfig()

    # Override device serial
    config.device.serial = serial

    # Create and run agent
    agent = DroidAgent(config=config)
    print(f"[+] Running DroidRun agent: {goal}")
    result = await agent.run(goal)

    print("\n[+] Result:")
    print(result)
    return result


def _run_droidrun_cli(goal: str, serial: str, config_path: str | None):
    """Fallback: run droidrun via CLI."""
    cmd = ["python3", "-m", "droidrun.cli.main", "run"]
    cfg = config_path or DROIDRUN_CONFIG
    if os.path.exists(cfg):
        cmd.extend(["--config", cfg])
    cmd.extend(["--serial", serial, goal])
    subprocess.run(cmd)


def main():
    parser = argparse.ArgumentParser(
        description="CyberStrikeAI DroidRun-Cuttlefish Bridge"
    )
    parser.add_argument("goal", help="Natural language goal for the agent")
    parser.add_argument("--install", "-i", help="APK to install before running")
    parser.add_argument("--config", "-c", help="DroidRun config YAML path")
    parser.add_argument("--serial", "-s", help="ADB device serial (auto-detect if omitted)")
    parser.add_argument("--no-portal", action="store_true", help="Skip Portal APK setup")
    parser.add_argument("--launch", action="store_true", help="Force launch Cuttlefish first")
    args = parser.parse_args()

    # Ensure device is running
    if args.serial:
        serial = args.serial
    else:
        serial = ensure_device_running()

    # Install Portal APK
    if not args.no_portal:
        install_portal(serial)

    # Install target APK if provided
    if args.install:
        install_apk(args.install, serial)

    # Run the agent
    asyncio.run(run_droidrun(args.goal, serial, args.config))


if __name__ == "__main__":
    main()
