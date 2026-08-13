---
name: competition-stego-media
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for image, audio, video, document, and container steganography. Use when the user
  asks to inspect metadata, alpha or palette channels, LSBs, thumbnails, appended
  trailers, QR fragments, transcoding artifacts, or recover a hidden payload from
  media without blind brute force. Use only after `$ctf-sandbox-orchestrator` has
  already established sandbox assumptions and routed here.
---

# Competition Stego Media

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the challenge lives inside a media container, hidden channel, or appended payload rather than a conventional crypto blob.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Confirm the real container type, dimensions, duration, codec, and chunk layout before guessing a hidden layer.
2. Check metadata, thumbnails, sidecar files, and appended trailers before deeper signal-domain work.
3. Rank candidate channels by evidence: alpha, palette, LSB, transform-domain residue, frame order, or container slack.
4. Preserve each extracted layer separately so the transform chain stays reproducible.
5. Stop when the hidden payload is reproduced, not merely suspected.

## Workflow

### 1. Establish Container Truth

- Inspect headers, chunk tables, EXIF or document metadata, container indexes, thumbnails, and file size anomalies.
- Compare declared format against observed structure to catch polyglots, appended archives, or malformed trailers.
- Record exact offsets, frame numbers, or channel boundaries that look promising.

### 2. Inspect Candidate Channels

- Check alpha, palette order, RGB or YUV planes, LSBs, spectrogram features, document object streams, or video frame deltas.
- Prefer evidence-driven attempts over brute forcing every transform.
- Note whether the payload is plain bytes, another media layer, compressed data, or an encrypted blob.

### 3. Reconstruct The Hidden Payload Path

- Keep the chain in order: container -> channel or carrier -> extraction -> decompression or decode -> final parse.
- Separate extraction success from final interpretation; a channel hit is not the same as artifact recovery.
- If the problem becomes primarily about cryptography after extraction, hand off to the broader crypto skill.

## Read This Reference

- Load `references/stego-media.md` for the media checklist, channel ranking guide, and evidence packaging.

## What To Preserve

- File structure facts: offsets, chunks, frame numbers, stream names, metadata keys, and trailer size
- Intermediate extractions and the exact command or transform used to produce them
- The final recovered payload and the channel that produced it
