---
name: offensive-keylogger-arch
description: 'Low-level keylogger architecture design: kernel driver hooks (WH_KEYBOARD_LL,
  SetWindowsHookEx), ETW-based input capture, user-mode vs kernel-mode approaches,
  stealth techniques, and data exfiltration. Use for understanding input capture mechanisms,
  EDR evasion research, or malware architecture analysis.'
---

# SKILL: Novel research

## Metadata
- **Skill Name**: keylogger-architecture
- **Folder**: offensive-keylogger-arch
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/Low-level%20Keylogger%20architecture_.md

## Description
Low-level keylogger architecture design: kernel driver hooks (WH_KEYBOARD_LL, SetWindowsHookEx), ETW-based input capture, user-mode vs kernel-mode approaches, stealth techniques, and data exfiltration. Use for understanding input capture mechanisms, EDR evasion research, or malware architecture analysis.

## Trigger Phrases
Use this skill when the conversation involves any of:
`keylogger, keyboard hook, WH_KEYBOARD_LL, SetWindowsHookEx, ETW, kernel driver, input capture, low-level keylogger, malware architecture, stealth, exfiltration`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology



Case study of different keylogger implementations, how to implement them and their individual IOCs.

---
## SetWindowHookEx
Majority of malware uses user32.dll!SetWindowHookEx to create a global hook event. this modifies an internal structure in `win32k.sys`.
Internally, `SetWindowsHookEx` is just a user-mode wrapper around `NtUserSetWindowsHookEx` (which itself wraps around `zzzzNtUserSetWindowsHookEx`) in `win32k.sys`.  What happens after you call it depends on the **hook type** you request but the sequence is always the same four steps:

1. **Validate and allocate a hook record**  
   `win32k.sys` creates an internal `HOOK` structure, fills in the filter type, module handle, thread/desktop IDs, and inserts the structure at the **head of the global hook chain** for that type
2. **Decide whether the hook procedure must live in the target process**  
   - **Low-level hooks (`WH_KEYBOARD_LL`, `WH_MOUSE_LL`)**  
     – **NO** injection.  
     – The system leaves the hook DLL in the **original caller’s address space** and simply delivers the event to that process via an internal `WM_*` message posted to its **hidden “ghost” window** .  
   - **All other global hooks (`WH_KEYBOARD`, `WH_CBT`, `WH_GETMESSAGE`, …)**  
     – **YES** injection required.  
     – For every process that satisfies the filter (same desktop, matching bitness),
	   - In/before Vista: `win32k` queues an **asynchronous load request** to `csrss.exe`, which in turn calls `LoadLibraryEx` inside the target process, mapping the hook DLL and fixing up its entry point.
	   - After Vista: The target process is added to a **pending-load list** inside `win32k`; the **first user-mode exit** from kernel to that process takes the APC and calls `LdrLoadDll` directly.
     – The first time the target thread is about to return to user mode, the kernel **APCs** the loader, so the DLL’s `DllMain` runs in the context of the victim process.

3. **Event routing at runtime**  
   When the monitored event occurs (key press, window activation, etc.), `win32k` walks the hook chain **inside the thread that owns the input queue**.  
   - If the hook procedure lives in that process, the kernel simply **calls the address** inside the injected DLL.  
   - If the procedure lives in another process (low-level case), the kernel **marshals the raw parameters** (`KBDLLHOOKSTRUCT` / `MSLLHOOKSTRUCT`) into an internal message and posts it to the **installing thread’s message queue**.  
     That thread must keep pumping messages; otherwise, the system **blocks all further input** for the desktop, which is why low-level hooks are so easy to detect by their side-effect on system responsiveness.

4. **Mandatory `CallNextHookEx`**  
   Each hook handler **must** call `CallNextHookEx` to pass control down the chain.  
   Internally, `CallNextHookEx` is just a call back into `win32k`, which continues the chain walk; if any handler fails to call it, the chain is broken and subsequent handlers never run. This might break input for the whole session.
#### TLDR
- **Low-level hooks** look stealthy because **no foreign code is mapped**, but they **pin the installing thread** and are trivially detected by their **message-queue footprint**.  
- **Regular global hooks** achieve **true code injection** without `WriteProcessMemory` or `CreateRemoteThread`, but they **leave a mapped DLL** behind in every hooked process. Easy VAD artefact for EDRs.
	- most EDRs avoid exhaustive VAD walks for every process on every event due to performance, but many will do targeted scans on on suspicious events (allocation > 64 kB, RWX, etc.).
- The **hook chain is global per desktop**: once installed, your procedure sees **every qualifying event** on that desktop, which is why a single call can key-log the whole user session.
### IOCs:
- Could be caught by a hook in user32
- Additional entry in the VAD (EDRs can check if the DLL is signed),
- Mapped or on-disk DLL
	- Is it signed?
		- Memory scanners could detect non-backed-by-disk executable memory.
	- Does it have anything to do here?
	- Could be bypassed by ovewriting a present, mapped DLL with our memory?
		- Would need to prevent user from interacting with keyboard while it happens.


---
## NtUserSetWindowsHookEx / zzzzNtUserSetWindowsHookEx
Same as above but you're directly calling the lower-level function. Same IOCs, really. You're only bypassing potential hooks in user32.dll.
The full logic of these functions could be reimplemented fully without a jump to external modules but it has too much IOCs and is too complex to implement to really be interesting.

**Session boundary**: raw-input registration is **per-session**, not per-desktop.  
A service in session-0 **cannot** register for keyboard raw-input and expect to see session-1 keystrokes – the HID packets are **routed to the session that owns the target HWND**.  
(You **can** open the **physical keyboard device object** directly and parse HID, but that is a **completely different attack surface** – needs admin, bypasses win32k.)

### IOCs:
- Additional entry in the VAD (EDRs can check if the DLL is signed),
	- ^ only theorical. No EDR implements this afaik
- Mapped or on-disk DLL
	- Is it signed?
		- Memory scanners could detect non-backed-by-disk executable memory.
	- Does it have anything to do here?

---
### NtUserRegisterRawInputDevices / RegisterRawInputDevices

tells the window manager to **deliver raw HID packets** to **one specific HWND** (or to the thread whose queue the window is attached to)

Practical abuse scenario
1. Start a **background thread** in our process or implement a `PeekMessage` / `GetMessage` loop.
2. Create a **zero-sized message-only window** (`HWND_MESSAGE`).
3. Register keyboard raw-input with `RIDEV_INPUTSINK` – > this routes **all keyboard traffic** to our window **even when it is not in the foreground** .
4. Pump the thread’s message queue forever; in the `WM_INPUT` handler call `GetRawInputData` and log the `RAWKEYBOARD` payload.
5. exfil
6. Profit?

Because no hook is installed, this technique:
- does **not** appear in `WinDbg`’s `!hook` list
- leaves **no cross-process DLL mapping**
- is **invisible to most EDR “hook chain” sensors**

this **still requires your process to stay alive and message-aware**, and it **cannot key-log from sessions it is not running in**.

Kernel-mode implementation:
1. Sets an oplock to prevent race conditions
2. Validates parameters
3. `Win32AllocPoolWithQuotaZInit`  
    Allocates a **kernel copy** of the array
4. `RegisterRawInputDevices(v9, a2, 0)`
    Calls the **INTERNAL worker** (see below).  
    It walks the array, updates the **per-thread raw-input hook list**,  
    tells **hidclass** which top-level windows want raw HID traffic, etc.
5. `EtwTraceAuditApiRegisterRawInputDevices`  
    Emits an **ETW** event for **Audit/Threat-Intelligence** so that defenders can see which process just asked for raw keyboard data (keylogger-style activity).
6. Cleanup

The internal worker modifies our process's EPROCESS structure. This makes it so that we can't re-implement this from user-mode.

### IOCs:
- Raises ETW event from kernel-mode win32kfull.sys driver.
	- **NOT AVOIDABLE!**
	- Do AVs/EDRs really monitor it though?
		- Rumors have it that Defender does since 20H1.
	- The ETW payload contains **PID, TID, UsagePage, Usage, Flags** – enough to **trivially score** “key-board raw-input from a non-interactive process” as **suspicious**.
	- Channel is **on by default** and **cannot be disabled** without patching the kernel.  
		→ **This is the strongest IOC** for this technique; **do not discount it**.
- **Raw-input must have a window station and desktop** – the call **fails** (`ERROR_INVALID_WINDOW_HANDLE`) if the thread is **not connected to a desktop**.   Services running in session-0 with **no desktop** therefore **cannot** use this path; they **must** either:  
		– create a **hidden desktop** (logged by **Object Manager auditing**), or
		– open the **\Device\KeyboardClass0** device directly (creates **IRP_MJ_READ** telemetry).
		- Maybe less noisy?
	Both are **easy to alert on**.

---

## Capturing current window's name

To filter for interesting keystrokes you may only monitor keystrokes from Chrome.exe \ firefox.exe, etc.

Different methods of doing that:
### GetWindowTextA
- The most detected function ever, every skid keylogger calls it.
- Eventually wraps around `NtUserInternalGetWindowText`.
- Not much else to say.

### NtUserInternalGetWindowText
- Much less detected because its a very low-level function
- Same signature as **GetWindowTextW**
- Defined in `Win32kFull.sys`.
	- DLL: `win32u.dll`

Reverse-engineering this was very tedious because the only references of this online seem to be:
```C
BOOL InternalGetWindowText(HWND hwnd, LPWSTR pString, int cchMaxCount) {
	DWORD retval = (DWORD)NtUserInternalGetWindowText(hwnd, pString, cchMaxCount);
	if (!retval) {
        *pString = (WCHAR)0;
    }
	return retval
}
```

Consult [1](https://dl.malwarewatch.org/software/features/ntvdmx64/build/nt5docs/d0/d0/ntuser_8h.html), [2](https://dl.malwarewatch.org/software/features/ntvdmx64/build/nt5docs/d9/d8/client_2ntstubs_8c-source.html#l00926) for more

its a syscall so you can use your favorite \*gate technique on it

---

# Novel research

Now... that's all stuff that can be figured out by anyone determined
for the unique research... contact me @ lovestrangekz on tg, everything has a price :]


---
Ideas that were abandonned:
- Use `NtUserBuildHwndList`/`EnumWindows` and re-implement the z-order heuristic to generate the list of all handles to all windows and call IsWindowVisible on them and do some other stuff to figure out if they're foreground or not?
	- Abandonned because, while this works, this is so complex to implement and there's no reliable way of knowing if it's foreground from user-mode (check next point)

- Walk `_K_USER_SHARED_DATA` to query its `ConsoleSessionForegroundProcessId` member then query the system to know that PID's windows and hope it only has one
	- Abandonned because, as above, we can't really know if that window is in foreground,
	- doesn't help much if target PID has multiple window handles

---

lovestrange @ [TeamKavkaz](https://t.me/teamkavkaz25)
join our channel for more
hackerz 4 lyfe
