# CTF Forensics - Advanced Disk and Memory Techniques

## Table of Contents
- [Deleted Partition Recovery](#deleted-partition-recovery)
- [ZFS Forensics (Nullcon 2026)](#zfs-forensics-nullcon-2026)
- [GPT Partition GUID Data Encoding (VuwCTF 2025)](#gpt-partition-guid-data-encoding-vuwctf-2025)
- [Windows Minidump String Carving (0xFun 2026)](#windows-minidump-string-carving-0xfun-2026)
- [VMDK Sparse Parsing (0xFun 2026)](#vmdk-sparse-parsing-0xfun-2026)
- [Memory Dump String Carving (Pragyan 2026)](#memory-dump-string-carving-pragyan-2026)
- [Memory Dump Malware Extraction + XOR (VuwCTF 2025)](#memory-dump-malware-extraction--xor-vuwctf-2025)
- [Linux Ransomware Memory-Key Recovery (MetaCTF 2026)](#linux-ransomware-memory-key-recovery-metactf-2026)
- [WordPerfect Macro XOR Extraction (srdnlenCTF 2026)](#wordperfect-macro-xor-extraction-srdnlenctf-2026)
- [Minidump ISO 9660 Recovery + XOR Key (srdnlenCTF 2026)](#minidump-iso-9660-recovery--xor-key-srdnlenctf-2026)
- [APFS Snapshot Historical File Recovery (srdnlenCTF 2026)](#apfs-snapshot-historical-file-recovery-srdnlenctf-2026)
- [RAID 5 Disk Recovery via XOR (Crypto-Cat)](#raid-5-disk-recovery-via-xor-crypto-cat)
- [HFS+ Resource Fork Hidden Binary Recovery (CONFidence CTF 2017)](#hfs-resource-fork-hidden-binary-recovery-confidence-ctf-2017)
- [Kyoto Cabinet Hash Database Forensics via Incremental Key Insertion (ASIS CTF 2018)](#kyoto-cabinet-hash-database-forensics-via-incremental-key-insertion-asis-ctf-2018)
- [SQLite Edit History Reconstruction from Diff Table (Google CTF 2017)](#sqlite-edit-history-reconstruction-from-diff-table-google-ctf-2017)
- [See Also](#see-also)

---

## Deleted Partition Recovery

**Pattern (Till Delete Do Us Part):** USB image with deleted partition table.

**Recovery workflow:**
```bash
# Check for partitions
fdisk -l image.img              # Shows no partitions

# Recover partition table
testdisk image.img              # Interactive recovery

# Or use kpartx to map partitions
kpartx -av image.img            # Maps as /dev/mapper/loop0p1

# Mount recovered partition
mount /dev/mapper/loop0p1 /mnt/evidence

# Check for hidden directories
ls -la /mnt/evidence            # Look for .dotfolders
find /mnt/evidence -name ".*"   # Find hidden files
```

**Flag hiding:** Path components as flag chars (e.g., `/.Meta/CTF/{f/l/a/g}`)

---

## ZFS Forensics (Nullcon 2026)

**Pattern:** Corrupted ZFS pool image with encrypted dataset.

**Recovery workflow:**
1. **Label reconstruction:** All 4 ZFS labels may be zeroed. Find packed nvlist data elsewhere in the image using `strings` + offset searching.
2. **MOS object repair:** Copy known-good nvlist bytes to block locations, recompute Fletcher4 checksums:
```python
def fletcher4(data):
    a = b = c = d = 0
    for i in range(0, len(data), 4):
        a = (a + int.from_bytes(data[i:i+4], 'little')) & 0xffffffff
        b = (b + a) & 0xffffffff
        c = (c + b) & 0xffffffff
        d = (d + c) & 0xffffffff
    return (d << 96) | (c << 64) | (b << 32) | a
```
3. **Encryption cracking:** Extract PBKDF2 parameters (iterations, salt) from ZAP objects. GPU-accelerate with PyOpenCL for PBKDF2-HMAC-SHA1, verify AES-256-GCM unwrap on CPU.
4. **Passphrase list:** rockyou.txt or similar. GPU rate: ~24k passwords/sec.

---

## GPT Partition GUID Data Encoding (VuwCTF 2025)

**Pattern (Undercut):** "LLMs only" + "undercut" → not AI GPT, but GUID Partition Table.

**Key insight:** GPT partition GUIDs are 16 arbitrary bytes — can encode anything. Look for file magic headers in GUIDs.

```bash
# Parse GPT partition table
gdisk -l image.img
# Or with Python:
python3 -c "
import struct
data = open('image.img','rb').read()
# GPT header at LBA 1 (offset 512)
# Partition entries start at LBA 2 (offset 1024)
# Each entry is 128 bytes, GUID at offset 16 (16 bytes)
for i in range(128):
    entry = data[1024 + i*128 : 1024 + (i+1)*128]
    guid = entry[16:32]
    if guid != b'\x00'*16:
        print(f'Partition {i}: {guid.hex()}')
"
```

**First GUID starts with `BZh11AY&SY`** (bzip2 magic) → concatenate GUIDs, decompress as bzip2, then decode ASCII85.

---

## Windows Minidump String Carving (0xFun 2026)

**Pattern (kd):** Go binary crash dump. Flag as plaintext string constant in .data section survives in minidump memory.

```bash
strings -a minidump.dmp | grep -i "flag\|ctf\|0xFUN"
```

**Lesson:** Minidumps contain full memory regions. String constants, keys, and secrets persist. `strings -a` + `grep` is the fast path.

---

## VMDK Sparse Parsing (0xFun 2026)

**Pattern (VMware):** Split sparse VMDK requires grain directory + grain table traversal.

**Key steps:**
1. Parse VMDK sparse header (grain size, GD offset, GT coverage)
2. Follow grain directory → grain table → data grains
3. Calculate absolute disk offsets across split files
4. Mount extracted filesystem (ext4, NTFS)

**Lesson:** Don't assume VM images can be mounted directly. Parse the VMDK sparse format manually.

---

## Memory Dump String Carving (Pragyan 2026)

**Pattern (c47chm31fy0uc4n):** Linux memory dump with flag in environment variables or process data.

```bash
strings -a -n 6 memdump.bin | grep -E "SYNC|FLAG|SSH_CLIENT|SESSION_KEY"
# SSH artifacts reveal source IP and ephemeral port
# Environment variables may contain keys/tokens
```

---

## Memory Dump Malware Extraction + XOR (VuwCTF 2025)

**Pattern (Jellycat):** Extract fake executable from Windows memory dump. Cipher: subtract 0x32, then XOR with cycling key (large multi-line string, e.g., ASCII art).

**Key lesson:** Always extract and reverse the actual binary from memory rather than trusting `strings` output (string tables may be red herrings). XOR keys can be hundreds of bytes (ASCII art, lorem ipsum).

```python
# Extract binary, find XOR key in data section
key = b"..."  # Large ASCII art string
cipher = open('extracted.bin', 'rb').read()
plaintext = bytes((b - 0x32) ^ key[i % len(key)] for i, b in enumerate(cipher))
```

---

## Linux Ransomware Memory-Key Recovery (MetaCTF 2026)

**Pattern:** Linux memory dump + encrypted `.veg` files + `enc_key.bin`; ransomware uses hybrid crypto (AES for files, RSA-wrapped key). Volatility may fail process enumeration due symbol/KASLR (Kernel Address Space Layout Randomization) mismatch.

**Fast workflow:**
1. **Confirm archive integrity before analysis.**
```bash
unzip -l encrypted_files.zip
# Compare listed files/sizes vs extracted tree; re-extract cleanly if mismatch
unzip -o encrypted_files.zip -d encrypted_full
```

2. **Reverse ransomware binary quickly to identify mode/layout.**
```bash
strings -a ransomware.elf | grep -E "enc_key|EVP_aes|PUBLIC KEY|.veg"
objdump -d ransomware.elf | less
```
- Typical finding: `AES-256-OFB`, IV prepended to each `.veg`, global 32-byte AES key, RSA public key hardcoded.

3. **Try Volatility normally, then pivot immediately if empty/unstable.**
```bash
vol -f memdump.raw linux.pslist
vol -f memdump.raw linux.proc.Maps
vol -f memdump.raw linux.vmayarascan
```
- If Linux plugins return empty/invalid output despite correct banner/symbols, do **raw-memory candidate scanning**.

4. **Recover AES key via anchored candidate scan + magic validation.**
- Use recurring anchor strings in memory (e.g., `/home/.../enc_key.bin`, HOME path).
- Derive candidate offsets near anchors (page-aligned windows).
- Test each 32-byte candidate by decrypting first blocks of multiple `.veg` files and checking magic bytes (`%PDF-`, `PK\x03\x04`, `\x89PNG\r\n\x1a\n`).
- Keep candidates that satisfy multiple independent signatures.

5. **Decrypt full dataset and verify output completeness.**
```bash
# OFB: iv = first 16 bytes, ciphertext starts at +16
# Decrypt all *.veg recursively from a clean extraction directory
```
- Validate recovered file count against zip listing.
- Watch for duplicated mirror trees (e.g., `snap/*/Downloads/...`) and deduplicate logically.

6. **Defend against false flags.**
- Treat metadata-only flags as suspicious until corroborated by challenge context.
- Prefer tokens from primary project artifacts and perform uniqueness checks:
```bash
rg -n -a '[A-Za-z]+CTF\\{[^}]+\\}' recovered_full
pdftotext recovered_full/**/*.pdf - 2>/dev/null | rg '[A-Za-z]+CTF\\{'
```

**Key lessons:**
- Don't trust a partial/stale extraction tree; re-extract zip cleanly.
- In OFB ransomware, magic-byte validation is a fast key oracle.
- A plausible `CTF{...}` in metadata can be a decoy; confirm with corpus-wide consistency.

---

## WordPerfect Macro XOR Extraction (srdnlenCTF 2026)

**Pattern (Trilogy of Death Vol I: Corel):** Corel Linux disk image containing WordPerfect macro file (fc.wcm) with XOR-encrypted byte arrays.

**Key insight:** WordPerfect macro files (`.wcm`) can contain executable macros with embedded encrypted data. The XOR formula `(bb + kb) - 2*(bb & kb)` is mathematically equivalent to bitwise XOR.

**Brute-force 4-byte XOR key under charset constraints:**
```python
import string

docbody = [206, 56, 8, 128, 209, 47, 2, 149, ...]  # encrypted bytes from macro
allowed = set(map(ord, string.ascii_lowercase + string.digits + "_{}"))

# Find valid key bytes independently for each position mod 4
cands = []
for j in range(4):
    good = []
    for k in range(256):
        if all((docbody[i] ^ k) in allowed for i in range(j, len(docbody), 4)):
            good.append(k)
    cands.append(good)

# Try all combinations (usually very few candidates per position)
for k0 in cands[0]:
    for k1 in cands[1]:
        for k2 in cands[2]:
            for k3 in cands[3]:
                key = [k0, k1, k2, k3]
                pt = ''.join(chr(c ^ key[i % 4]) for i, c in enumerate(docbody))
                if pt.startswith("srd") and pt.endswith("}"):
                    print(pt)
```

**Lesson:** Legacy document formats (WordPerfect, Lotus 1-2-3) can embed executable macros with obfuscated data. When you know the flag charset, brute-forcing a short XOR key is trivial by filtering each key byte independently.

---

## Minidump ISO 9660 Recovery + XOR Key (srdnlenCTF 2026)

**Pattern (Trilogy of Death Vol II: The Legendary Armory):** Two relics in volatile memory (minidump) must be XORed; ISO 9660 directory entries in memory fragments point to hidden data.

**Technique:**
1. Search minidump for ISO 9660 directory entry signatures
2. Parse directory entries to locate target file offset and size
3. Decrypt file using recovered XOR key (e.g., 8-byte repeating key)
4. Parse resulting data as ZIP without central directory (local headers only)

**ZIP local header parsing without central directory:**
```python
import struct, zlib

pos = 0
files = {}
while True:
    off = dec.find(b"PK\x03\x04", pos)
    if off < 0:
        break
    (ver, flag, method, _, _, crc, csize, usize, nlen, xlen) = struct.unpack_from(
        "<HHHHHIIIHH", dec, off + 4)
    name = dec[off + 30:off + 30 + nlen].decode()
    data_off = off + 30 + nlen + xlen
    comp = dec[data_off:data_off + csize]
    if method == 8:  # Deflate
        raw = zlib.decompress(comp, -15)
    else:
        raw = comp
    files[name] = raw
    pos = data_off + csize
```

**Key insight:** When ZIP central directory is missing/corrupt, iterate local file headers (`PK\x03\x04`) directly. Each local header contains enough metadata (compression method, sizes, filename) to extract files independently.

---

## APFS Snapshot Historical File Recovery (srdnlenCTF 2026)

**Pattern (Trilogy of Death Vol III: The Poisoned Apple):** APFS volume maintains historical snapshots; recovering earlier state of a key file reveals authentic value before poisoning.

**Technique:**
1. Extract APFS partition from DMG (locate by sector offset)
2. Search for APFS volume superblocks (magic `APSB`) across all snapshots, noting transaction IDs (XIDs)
3. Use `icat` (Sleuth Kit with APFS support) to read specific inodes across different snapshot XIDs
4. Compare file content across XID boundaries to identify when poisoning occurred
5. Use pre-poisoning value for decryption

**Finding APFS volume superblocks across snapshots:**
```python
import struct

with open("apfs_partition.img", "rb") as f:
    mm = f.read()

snaps = []
pos = 0
while True:
    idx = mm.find(b"APSB", pos)
    if idx < 0:
        break
    # XID is at offset -16 from magic (in block header)
    hdr_start = idx - 32
    xid = struct.unpack_from("<Q", mm, hdr_start + 16)[0]
    blk = hdr_start // 4096
    snaps.append((xid, blk))
    pos = idx + 1

# Read target inode across snapshots
import subprocess
for xid, blk in sorted(set(snaps)):
    try:
        out = subprocess.check_output(
            ["icat", "-f", "apfs", "-P", "apfs", "-B", str(blk),
             "apfs_partition.img", "449414"])  # target inode number
        print(f"XID {xid}: {out[:64]}...")
    except:
        pass
```

**Decryption with recovered authentic key:**
```python
import hashlib
from Cryptodome.Cipher import AES

# Pre-poisoning key value (found in earlier snapshot)
authentic_key_hex = "39f520679fd68654500f9cd44e8caed2bc897a3227dc297c4520336de2a59dd7"
key = hashlib.pbkdf2_hmac('sha256', bytes.fromhex(authentic_key_hex), salt, iterations)
cipher = AES.new(key, AES.MODE_CBC, iv)
plaintext = cipher.decrypt(encrypted_flag)
```

**Key insight:** APFS (and other copy-on-write filesystems like ZFS/Btrfs) preserve historical file states in snapshots. When a challenge involves "poisoned" or "tampered" data, always check for older snapshots containing the original values. Use `icat` with different block offsets to read the same inode across different transaction IDs.

---

## RAID 5 Disk Recovery via XOR (Crypto-Cat)

**Pattern:** RAID 5 array with one damaged/missing disk. Two working disks are provided and the third must be reconstructed using XOR parity.

**How RAID 5 parity works:** Data is striped across N disks with distributed parity. For any stripe, `Disk1 XOR Disk2 XOR ... XOR DiskN = 0`. If one disk is missing, XOR the remaining disks to recover it.

**Recovery script:**
```python
# Recover missing disk2 from disk1 and disk3
with open('disk1.img', 'rb') as f:
    disk1 = f.read()
with open('disk3.img', 'rb') as f:
    disk3 = f.read()

# XOR byte-by-byte to recover the missing disk
disk2 = bytes(a ^ b for a, b in zip(disk1, disk3))

with open('disk2.img', 'wb') as f:
    f.write(disk2)
```

**After recovery:**
```bash
# Reassemble the RAID array
mdadm --create /dev/md0 --level=5 --raid-devices=3 \
  disk1.img disk2.img disk3.img

# Or mount individual recovered disk if it contains a filesystem
mount -o loop,ro disk2.img /mnt/recovered
```

**Key insight:** RAID 5 uses XOR parity across all disks in each stripe. XOR is self-inverse: if `A XOR B XOR C = 0`, then `B = A XOR C`. For N-disk RAID 5, XOR all N-1 working disks together to recover the missing one.

**Detection:** Challenge provides multiple disk images of identical size, mentions "array", "redundancy", or "parity". `file` command may identify them as filesystem images or raw data.

---

## HFS+ Resource Fork Hidden Binary Recovery (CONFidence CTF 2017)

HFS+ files can have a Resource Fork containing hidden data invisible to most tools. Use HFSExplorer to inspect the catalog and 010 Editor with HFS template to extract.

```bash
# 1. Mount or open the HFS+ image
# Standard tools miss Resource Forks:
binwalk image.dmg    # Won't find resource fork contents
strings image.dmg    # May show fragments

# 2. Use HFSExplorer to browse the catalog
# Look for files with non-zero Resource Fork size
# Suspicious: nodeID 1337 or similar CTF-typical IDs

# 3. Check .fseventsd logs for historical file operations
pip install FSEventsParser
python FSEventsParser.py -s image.dmg -o events.csv
# Reveals creation/deletion of files across the volume

# 4. Extract Resource Fork data with 010 Editor:
# - Load disk image with HFS+ template
# - Navigate to catalog -> target file -> resource fork extents
# - Note start block and length from extent records
# - If split across multiple extents, extract and concatenate:
dd if=image.dmg bs=4096 skip=$BLOCK1 count=$LEN1 of=part1.bin
dd if=image.dmg bs=4096 skip=$BLOCK2 count=$LEN2 of=part2.bin
cat part1.bin part2.bin > recovered_binary
```

**Key insight:** HFS+ Resource Forks are a second data stream attached to files, invisible to most forensic tools that only examine the Data Fork. `binwalk`, `foremost`, and `strings` miss them. HFSExplorer shows both forks in the catalog; 010 Editor with the HFS template reveals extent records for manual extraction. `.fseventsd` logs can reveal that hidden files were created/deleted.

**Detection:** DMG or HFS+ disk image where standard carving finds nothing. `file` identifies as "Apple HFS+" or "Apple Partition Map". Challenge mentions "Mac", "Apple", or "hidden data".

---

## Kyoto Cabinet Hash Database Forensics via Incremental Key Insertion (ASIS CTF 2018)

**Pattern:** Unknown binary file identified as Kyoto Cabinet (KC) hash database. Flag characters stored as values with zeroed-out keys. Since the database uses a fixed-size hash table, recover ordering by inserting sequential keys one at a time and observing which hash slot reference gets overwritten via binary diff.

```bash
# Identify format
file unknown.db  # may not recognize KC format
strings unknown.db | head  # look for "KCPH" magic

# Enumerate values
kchashmgr list tokyo.kch

# Recover key ordering via incremental insertion + binary diff
for i in $(seq -w 000 088); do
    cp tokyo.kch test.kch
    kchashmgr set test.kch "$i" "probe"
    diff <(xxd tokyo.kch) <(xxd test.kch) | head -5
    # Changed offset reveals which original entry maps to key $i
done
```

**Full recovery script (Python):**
```python
import subprocess, shutil

original = 'tokyo.kch'
# Get all values from the database
values = subprocess.check_output(['kchashmgr', 'list', original]).decode().splitlines()

mapping = {}
for i in range(len(values)):
    key = f'{i:03d}'
    shutil.copy(original, 'test.kch')
    subprocess.run(['kchashmgr', 'set', 'test.kch', key, 'probe'], check=True)
    # Binary diff to find which slot changed
    orig_hex = subprocess.check_output(['xxd', original]).decode()
    test_hex = subprocess.check_output(['xxd', 'test.kch']).decode()
    for orig_line, test_line in zip(orig_hex.splitlines(), test_hex.splitlines()):
        if orig_line != test_line:
            mapping[i] = orig_line  # Record which entry was overwritten
            break

# Reconstruct flag from ordered values
flag = ''.join(values[i] for i in sorted(mapping.keys()))
print(flag)
```

**Key insight:** Hash databases store entries at positions determined by key hash values. When keys are zeroed/corrupted, the stored ordering is hash-based, not insertion-order. Insert probe keys one at a time and binary-diff the database to find which slot each probe overwrites, revealing the original key-to-value mapping.

---

## SQLite Edit History Reconstruction from Diff Table (Google CTF 2017)

SQLite databases storing note/document edit history as diff entries (operation, position, text, diffset) can be replayed to reconstruct content at any point in time.

```python
import sqlite3

db = sqlite3.connect('notes.db')
# Table structure: diffs(id, type, position, text, diffset)
# type: 'insert' or 'remove'
diffs = db.execute("SELECT type, position, text FROM diffs ORDER BY id").fetchall()

document = ""
for op_type, position, text in diffs:
    if op_type == 'insert':
        document = document[:position] + text + document[position:]
    elif op_type == 'remove':
        document = document[:position] + document[position + len(text):]
    # Check for flag at each step (may have been typed then deleted)
    if 'CTF{' in document or 'flag{' in document:
        print(f"Flag found: {document}")
```

**Key insight:** Collaborative editing tools store incremental diffs. Replaying all operations sequentially reveals content that existed at any point in the edit history, including secrets that were later deleted. Check for flags at every intermediate state, not just the final document.

**Detection:** SQLite database with tables containing `type`/`operation`, `position`, `text` columns. Challenge mentions "notes", "editor", "collaboration", or "history". Schema inspection via `.schema` or `sqlite3 db.sqlite ".tables"` reveals diff-style tables.

---

## See Also

- [disk-and-memory.md](disk-and-memory.md) - Core disk and memory forensics (Volatility 3, disk image analysis, VM/OVA/VMDK forensics, VMware snapshots, GIMP raw memory dump visual inspection, coredump analysis, Windows KAPE triage, PowerShell ransomware, Android forensics, Docker container forensics, cloud storage forensics, BSON reconstruction, TrueCrypt/VeraCrypt mounting)
- [disk-recovery.md](disk-recovery.md) - Disk recovery and extraction patterns (LUKS master key recovery, PRNG timestamp seed brute-force, VBA macro binary recovery, FemtoZip decompression, XFS reconstruction, tar duplicate entry extraction, nested matryoshka filesystem extraction, anti-carving via null byte interleaving)
