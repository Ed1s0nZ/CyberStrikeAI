---
name: competition-firmware-layout
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for firmware images, partition tables, boot chains, update packages, extracted filesystems,
  embedded configs, and device-facing trust boundaries. Use when the user asks to
  unpack firmware, map partition layout, inspect bootloader or init chains, recover
  update keys or credentials, trace config loading, or explain how a device surface
  reaches the decisive artifact. Use only after `$ctf-sandbox-orchestrator` has already
  established sandbox assumptions and routed here.
---

# Competition Firmware Layout

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the hard part is understanding how a firmware image is structured, booted, updated, and turned into reachable device behavior.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Preserve the original image, extracted partitions, unpacked filesystems, and patched copies as separate artifacts.
2. Map outer container, partition table, bootloader, kernel, rootfs, config, and update metadata before editing anything.
3. Track the boot or update chain in order instead of jumping straight to the most interesting file.
4. Record keys, signatures, offsets, partition boundaries, and init entrypoints in one compact evidence chain.
5. Reproduce the decisive secret, branch, or reachable service from the smallest extracted path.

## Workflow

### 1. Establish Image Layout

- Identify container type, partition headers, compression, filesystem type, and any appended or nested images.
- Record offsets, sizes, hashes, mount points, and partition names before extraction mutates anything.
- Separate bootloader, kernel, initramfs, rootfs, config blobs, and update metadata as different layers.

### 2. Trace Boot Or Update Flow

- Map how control moves from bootloader to kernel to init to services, or from update package to verifier to installer.
- Note which credentials, certificates, passwords, seeds, or config files are consumed at each stage.
- Distinguish checked-in firmware intent from the live behavior the extracted files actually support.

### 3. Reduce To The Decisive Path

- Show the smallest chain from image boundary to service exposure, auth bypass, debug interface, credential recovery, or flag artifact.
- Keep extracted filesystems, derived configs, and patch experiments separate from pristine inputs.
- If the challenge becomes mostly about native crash behavior or exploit primitives after extraction, switch back to the broader reverse skill.

## Read This Reference

- Load `references/firmware-layout.md` for the layout checklist, boot-chain checklist, and evidence packaging.

## What To Preserve

- Partition offsets, hashes, filesystem types, mount paths, boot entrypoints, and update metadata
- Extracted secrets, config paths, init scripts, service units, and credentials tied to the stage that consumes them
- Original images, extracted layers, mounted views, and patched copies as separate artifacts
