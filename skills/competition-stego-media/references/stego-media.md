# Stego Media Checklist

## First Pass

- Verify magic bytes, headers, chunk or atom layout, dimensions, duration, sample rate, and metadata
- Check for appended data, malformed trailers, duplicate thumbnails, embedded archives, or sidecar files
- Record entropy shifts, unexpected padding, or container slack regions

## Candidate Channels

1. Alpha or palette anomalies
2. LSBs in image or audio samples
3. Frame-order or delta anomalies in video
4. Document object streams, attachments, or hidden layers
5. Transform-domain residue, QR fragments, or watermark-style carriers

## Evidence Packaging

- Keep one compact block for offsets, channels, and extraction commands
- Keep each extracted stage as a separate file
- Note clearly whether the result is plaintext, another container, compressed data, or ciphertext

## Common Pitfalls

- Jumping into brute force before checking trailers and metadata
- Mixing several partial decode attempts without tracking which channel produced which bytes
- Treating a suspicious pattern as success without reproducing the hidden payload
