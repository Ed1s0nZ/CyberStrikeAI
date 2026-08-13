# CTF Forensics - Disk Recovery and Extraction Patterns

## Table of Contents
- [LUKS Master Key Recovery from Memory Dump (Hack.lu 2015)](#luks-master-key-recovery-from-memory-dump-hacklu-2015)
- [PRNG Timestamp Seed Brute-Force for Encryption Key Recovery (CSAW 2015)](#prng-timestamp-seed-brute-force-for-encryption-key-recovery-csaw-2015)
- [VBA Macro Encoded Binary Recovery (Sharif CTF 2016)](#vba-macro-encoded-binary-recovery-sharif-ctf-2016)
- [FemtoZip Shared Dictionary Decompression (Sharif CTF 2016)](#femtozip-shared-dictionary-decompression-sharif-ctf-2016)
- [XFS Filesystem Reconstruction from Corrupted Metadata (BSidesSF 2025)](#xfs-filesystem-reconstruction-from-corrupted-metadata-bsidessf-2025)
- [Tar Archive Duplicate Entry Extraction (BSidesSF 2025)](#tar-archive-duplicate-entry-extraction-bsidessf-2025)
- [Nested Matryoshka Filesystem Extraction (BSidesSF 2025)](#nested-matryoshka-filesystem-extraction-bsidessf-2025)
- [Anti-Carving via Null Byte Interleaving (BSidesSF 2024)](#anti-carving-via-null-byte-interleaving-bsidessf-2024)
- [BTRFS Subvolume/Snapshot Recovery (BSidesSF 2026)](#btrfs-subvolumesnapshot-recovery-bsidessf-2026)
- [FAT16 Free Space Data Recovery (BSidesSF 2026)](#fat16-free-space-data-recovery-bsidessf-2026)
- [FAT16 Deleted File Recovery via Sleuth Kit (MetaCTF Flash 2026)](#fat16-deleted-file-recovery-via-sleuth-kit-metactf-flash-2026)
- [Ext2 Orphaned Inode Recovery via fsck (BSidesSF 2026)](#ext2-orphaned-inode-recovery-via-fsck-bsidessf-2026)
- [Corrupted ZIP Repair via Header Field Manipulation (PlaidCTF 2017)](#corrupted-zip-repair-via-header-field-manipulation-plaidctf-2017)
- [Recovering Deleted .git Repository from FAT Image (Square CTF 2017)](#recovering-deleted-git-repository-from-fat-image-square-ctf-2017)
- [DNSSEC Key Recovery from Git Commit History (Hack.lu 2017)](#dnssec-key-recovery-from-git-commit-history-hacklu-2017)
- [XZ Stream Header Repair via CRC32 Reconstruction (Hackover 2018)](#xz-stream-header-repair-via-crc32-reconstruction-hackover-2018)
- [ZipCrypto Known-Plaintext Cracking via bkcrack (Codegate 2019)](#zipcrypto-known-plaintext-cracking-via-bkcrack-codegate-2019)
- [SQLite Serial-Type Byte Forensics (RITSEC 2018)](#sqlite-serial-type-byte-forensics-ritsec-2018)
- [Recursive Binwalk Chain PNG->PDF->DOCX->PNG->Base64 (TAMUctf 2019)](#recursive-binwalk-chain-png-pdf-docx-png-base64-tamuctf-2019)
- [Regex-Password Nested Zip Chain with exrex (UTCTF 2019)](#regex-password-nested-zip-chain-with-exrex-utctf-2019)
- [See Also](#see-also)

---

## LUKS Master Key Recovery from Memory Dump (Hack.lu 2015)

Recover LUKS encryption keys from VM memory dumps using AES key schedule detection:

1. **Extract memory:** Obtain memory dump from VM snapshot (.elf, .vmem, .raw)
2. **Find AES keys:** Use `aeskeyfind` to detect AES key schedules in memory

```bash
aeskeyfind memory.elf
# Output: candidate AES-256 keys (64 hex chars each)
```

3. **Write key to file:** Convert hex key to binary

```bash
echo "deadbeef..." | xxd -r -p > master.key
```

4. **Add new LUKS passphrase using master key:**

```bash
cryptsetup luksAddKey --master-key-file master.key /dev/mapper/volume
# Enter new passphrase when prompted
cryptsetup luksOpen /dev/mapper/volume decrypted
mount /dev/mapper/decrypted /mnt
```

**Key insight:** AES key schedules have a distinctive mathematical structure that `aeskeyfind` detects regardless of where they appear in memory. Works for LUKS, dm-crypt, FileVault, and BitLocker volumes.

Companion tools: `rsakeyfind` (RSA keys), `aesfix` (corrupted key recovery).

---

## PRNG Timestamp Seed Brute-Force for Encryption Key Recovery (CSAW 2015)

When encryption keys are generated from PRNG seeded with timestamps, brute-force the seed:

1. **Identify seed source:** Look for `Time.now.to_i`, `time(NULL)`, `System.currentTimeMillis()` used as PRNG seed
2. **Determine time window:** Use file metadata (creation/modification timestamps) to bound the search
3. **Brute-force seeds:** Try each second in a +/-24 hour window around the file timestamp

```python
import struct
from Crypto.Cipher import AES

# Ruby-compatible Random implementation (or use ctypes for C rand)
for seed in range(timestamp - 86400, timestamp + 86400):
    rng = RandomWithSeed(seed)
    key = bytes([rng.rand(256) for _ in range(32)])  # AES-256
    iv = bytes([rng.rand(256) for _ in range(16)])

    cipher = AES.new(key, AES.MODE_CBC, iv)
    plaintext = cipher.decrypt(ciphertext)

    # Validate: check for known file signatures
    if plaintext[:4] == b'\x89PNG' or plaintext[:2] == b'\xff\xd8':
        print(f"Found key with seed: {seed}")
        break
```

**Key insight:** Expand the time window beyond the obvious timestamp -- clock skew, timezone differences, and filesystem granularity can shift the effective seed by hours.

---

## VBA Macro Encoded Binary Recovery (Sharif CTF 2016)

Excel/Word macros may encode binary data in cell values. Extract and decode:

1. **Extract macro:** Use `olevba` or open in LibreOffice to inspect VBA code
2. **Identify encoding:** Look for cell iteration patterns like `Cells(i, j).Value`
3. **Reverse the encoding formula:**

```python
# If macro encodes as: cell_value = byte_value * 3 + 78
# Reverse: byte_value = (cell_value - 78) // 3

import openpyxl
wb = openpyxl.load_workbook('challenge.xlsx')
ws = wb.active

binary_data = bytearray()
for row in ws.iter_rows():
    for cell in row:
        if cell.value is not None:
            binary_data.append((int(cell.value) - 78) // 3)

with open('recovered.elf', 'wb') as f:
    f.write(binary_data)
```

**Key insight:** Check the recovered file with `file` command -- common outputs are ELF binaries, PE executables, or images containing the flag.

---

## FemtoZip Shared Dictionary Decompression (Sharif CTF 2016)

FemtoZip uses a shared dictionary model for compressing corpora of similar documents. When given a `.model` file and compressed data:

```bash
# Install femtozip
git clone https://github.com/gtoubassi/femtozip
cd femtozip && make

# Decompress using provided model
./fzip --model fashion.model --decompress compressed_dir/ --output decompressed_dir/
```

After decompression, search through potentially thousands of files:

```bash
# Filter by metadata fields
grep -r "category.*forensic" decompressed_dir/ | grep "year.*2016"
```

**Key insight:** FemtoZip is rare in CTFs. Identify it by the `.model` file and the presence of many small compressed files that share common structure (JSON, XML templates).

---

## XFS Filesystem Reconstruction from Corrupted Metadata (BSidesSF 2025)

When XFS superblock or allocation group metadata is corrupted but inodes are intact:

1. **Parse inode directly:** XFS inodes contain extent lists with `[startoff, startblock, blockcount]` tuples
2. **Calculate block offsets:** Multiply startblock by filesystem block size (typically 4K)
3. **Extract file data:** Copy blocks directly from the raw disk image

```bash
# Extract file from known inode extent
# startblock=104333, blockcount=256, block_size=4096
dd if=disk.img bs=4096 skip=104333 count=256 of=recovered.jpg

# Parse XFS inode structure (at known offset)
python3 -c "
import struct
with open('disk.img', 'rb') as f:
    f.seek(inode_offset)
    magic = f.read(2)  # 'IN' = 0x494e
    # Parse di_core (96 bytes): mode, uid, gid, nlink, size, etc.
    # Parse extent list: each extent = 16 bytes
    # startoff (54 bits) | startblock (52 bits) | blockcount (21 bits)
"
```

**Key insight:** XFS stores extent maps inline in the inode (up to ~4 extents). For files with more extents, follow the B+tree root in the inode. Use `xfs_db` if available: `xfs_db -r disk.img` → `inode <num>` → `print`.

---

## Tar Archive Duplicate Entry Extraction (BSidesSF 2025)

Tar format allows multiple entries with the same filename. Standard extraction overwrites earlier entries, but specific occurrences can be targeted:

```bash
# List all entries (shows duplicates)
tar -tvf archive.tar.xz | grep -c '^\.'

# Extract specific occurrence (1-indexed)
tar -Jxvf archive.tar.xz '.' --occurrence=2 -O > second_entry.bin

# Extract all occurrences via file carving
binwalk -e archive.tar
# Or iterate programmatically
python3 -c "
import tarfile
with tarfile.open('archive.tar.xz') as tf:
    for i, member in enumerate(tf.getmembers()):
        if member.name == '.':
            data = tf.extractfile(member).read()
            with open(f'entry_{i}.bin', 'wb') as f:
                f.write(data)
"
```

**Key insight:** The `--occurrence=N` flag in GNU tar selects the Nth entry with a matching name. Without it, only the last entry survives extraction. Challenges may hide flags in middle entries that normal extraction skips.

---

## Nested Matryoshka Filesystem Extraction (BSidesSF 2025)

Disk images containing nested compressed filesystem layers (potentially 10-20+ levels deep):

```bash
#!/bin/bash
# Automated layer extraction
IMG="disk.img"
for i in $(seq 1 20); do
    echo "=== Layer $i ==="
    file "$IMG"

    # Detect and decompress
    case "$(file -b "$IMG")" in
        *XZ*)     xz -d "$IMG"; IMG="${IMG%.xz}" ;;
        *gzip*)   gunzip "$IMG"; IMG="${IMG%.gz}" ;;
        *ext4*)
            mkdir -p "layer_$i"
            sudo mount -o ro,loop "$IMG" "layer_$i"
            IMG=$(find "layer_$i" -type f -name "*.img" -o -name "*.xz" | head -1)
            ;;
        *ISO*|*HFS*|*XFS*|*AmigaDOS*)
            mkdir -p "layer_$i"
            sudo mount -o ro,loop "$IMG" "layer_$i" 2>/dev/null || \
            sudo mount -t affs -o ro,loop "$IMG" "layer_$i" 2>/dev/null
            IMG=$(find "layer_$i" -type f | head -1)
            ;;
    esac
done
```

Filesystem types encountered: ext4, XFS, HFS/HFS+, AFFS (AmigaDOS), FAT. Use `losetup` with `--offset` for partitioned images. Final layer typically contains an image or text file with the flag.

**Key insight:** Install uncommon filesystem drivers (`hfsplus`, `affs`) beforehand. Some layers require manual sector offset calculation when partition tables are absent.

---

## Anti-Carving via Null Byte Interleaving (BSidesSF 2024)

Files stored with null bytes inserted at every other position defeat magic-byte-based file carving tools (binwalk, foremost, scalpel):

1. **Identify anti-carving:** File carving finds nothing, but `xfs_db` or filesystem-level tools show the file exists with correct size
2. **Extract raw blocks:** Use filesystem extent information to locate file data

```bash
# XFS: find file extents
xfs_db -r disk.img -c 'inode <inum>' -c 'print'
# Extract extent data
dd if=disk.img bs=4096 skip=<startblock> count=<blockcount> of=raw.bin
```

3. **Remove interleaved null bytes:** Keep only even-positioned (or odd-positioned) bytes

```python
with open('raw.bin', 'rb') as f:
    data = f.read()
# Remove null bytes at odd positions
cleaned = bytes(data[i] for i in range(0, len(data), 2))
with open('recovered.png', 'wb') as f:
    f.write(cleaned)
```

```perl
# Perl one-liner equivalent
perl -0777 -pe 's/(.)./\1/gs' raw.bin > recovered.png
```

**Key insight:** When file carving fails but the filesystem metadata is intact, extract via block-level access and look for byte-level obfuscation patterns. Null byte interleaving doubles the file size — compare actual size vs expected size as a detection heuristic.

---

---

## BTRFS Subvolume/Snapshot Recovery (BSidesSF 2026)

**Pattern (turn-back-the-clock):** Deleted files on a BTRFS filesystem may persist in snapshots or alternate subvolumes. The default mount shows only the active subvolume, but backup snapshots contain historical file states.

**Recovery workflow:**
```bash
# 1. Set up loop device
sudo losetup /dev/loop0 challenge.img

# 2. List available subvolumes
sudo btrfs subvolume list /dev/loop0
# Output: ID 256 gen 7 top level 5 path @
#         ID 257 gen 5 top level 5 path @backup

# 3. Mount the default subvolume (may show deleted files as missing)
sudo mount /dev/loop0 /mnt/default
ls /mnt/default/  # Flag file missing

# 4. Mount the backup subvolume
sudo mount -o subvol=@backup /dev/loop0 /mnt/backup
ls /mnt/backup/   # Flag file present!
cat /mnt/backup/flag.txt

# 5. Alternative: mount by subvolume ID
sudo mount -o subvolid=257 /dev/loop0 /mnt/backup
```

**Key BTRFS commands for forensics:**
```bash
# Show filesystem info
btrfs filesystem show /dev/loop0

# List all subvolumes (including snapshots)
btrfs subvolume list -a /mnt

# Show snapshot details
btrfs subvolume show /mnt/@backup

# Find deleted subvolumes (orphaned)
btrfs-find-root /dev/loop0
```

**BTRFS snapshot types:**
- **Writable subvolumes:** `@`, `@home` — standard Ubuntu layout
- **Read-only snapshots:** Created by `btrfs subvolume snapshot -r` — immutable copies
- **Backup subvolumes:** `@backup`, `@snap-YYYYMMDD` — naming varies by tool (Timeshift, snapper)

**Key insight:** BTRFS is copy-on-write. Deleting a file from the active subvolume doesn't erase the data if a snapshot or alternate subvolume still references those blocks. Always enumerate all subvolumes with `btrfs subvolume list`. The `-o subvol=` mount option is the key to accessing non-default subvolumes.

**Detection:** `file disk.img` shows "BTRFS Filesystem". Challenge mentions "snapshots", "time travel", "turn back", or "recovery".

**References:** BSidesSF 2026 "turn-back-the-clock"

---

## FAT16 Free Space Data Recovery (BSidesSF 2026)

**Pattern (freeflag):** Data is hidden in the free (unallocated) clusters of a FAT16 filesystem. The mounted filesystem shows no suspicious files, but free clusters contain recoverable data.

```python
import struct

with open("disk.img", "rb") as f:
    # Read FAT16 boot sector
    f.seek(0)
    boot = f.read(512)
    bytes_per_sector = struct.unpack_from("<H", boot, 11)[0]
    sectors_per_cluster = boot[13]
    reserved_sectors = struct.unpack_from("<H", boot, 14)[0]
    num_fats = boot[16]
    sectors_per_fat = struct.unpack_from("<H", boot, 22)[0]
    root_entries = struct.unpack_from("<H", boot, 17)[0]

    cluster_size = bytes_per_sector * sectors_per_cluster
    fat_start = reserved_sectors * bytes_per_sector
    root_dir_start = fat_start + (num_fats * sectors_per_fat * bytes_per_sector)
    data_start = root_dir_start + (root_entries * 32)

    # Read FAT table
    f.seek(fat_start)
    fat = f.read(sectors_per_fat * bytes_per_sector)

    # Find free clusters (FAT entry == 0x0000)
    free_data = b""
    for cluster in range(2, len(fat) // 2):
        entry = struct.unpack_from("<H", fat, cluster * 2)[0]
        if entry == 0x0000:  # Free cluster
            offset = data_start + (cluster - 2) * cluster_size
            f.seek(offset)
            free_data += f.read(cluster_size)

    # Search for flag in free space
    if b"CTF{" in free_data:
        idx = free_data.index(b"CTF{")
        print(free_data[idx:idx+100])
```

**Key insight:** FAT16/FAT32 mark deleted file clusters as "free" (entry = 0x0000) but don't zero the data. Enumerating free clusters and reading their contents recovers deleted or hidden data. Tools like `foremost`, `scalpel`, or manual FAT parsing extract this data. Check the volume label for hints (e.g., "FREESPACE").

**When to recognize:** Challenge provides a filesystem image. Mounting shows nothing useful, but `file` identifies it as FAT16/FAT32. Volume label or challenge description hints at "free space", "deleted", or "hidden in plain sight".

**References:** BSidesSF 2026 "freeflag"

---

## FAT16 Deleted File Recovery via Sleuth Kit (MetaCTF Flash 2026)

**Pattern (rm -rf flag.png):** A file has been deleted from a FAT16 filesystem image. The file's data and cluster chain remain intact, but the directory entry's first byte is replaced with `0xE5` (the FAT deletion marker). Sleuth Kit's `fls` and `icat` recover the file by inode.

```bash
# Step 1: Identify the filesystem
file flash.img
# flash.img: DOS/MBR boot sector, code offset 0x3e+2, ... FAT (16 bit) ...

# Step 2: List all files including deleted ones (-d = deleted only, -r = recursive)
fls -r -d flash.img
# r/r * 4:    _lag.png    (first char replaced by FAT deletion marker)

# Step 3: Recover the deleted file by its inode number
icat flash.img 4 > recovered_flag.png

# Step 4: Verify recovery
file recovered_flag.png
# recovered_flag.png: PNG image data, 800 x 600, 8-bit/color RGBA
```

**Key insight:** FAT16/FAT32 deletion only marks the directory entry's first byte as `0xE5` and marks clusters as free in the FAT table, but the actual file data remains on disk until overwritten. The filename appears scrambled (e.g., `flag.png` becomes `_lag.png`), but `fls -d` lists deleted entries and `icat` extracts the full file by following the original cluster chain. This is more targeted than free space carving because it preserves the original file boundaries.

**When to recognize:** Challenge provides a FAT filesystem image with a deleted file. The challenge name or description hints at deletion (`rm`, `deleted`, `removed`). Mount shows the file is missing, but `fls` reveals the deleted directory entry.

**Alternative approaches:**
- `foremost` / `scalpel` for carving without filesystem awareness
- `fatcat` for low-level FAT manipulation
- Manual hex editing: search for `0xE5` entries in directory clusters

**References:** MetaCTF Flash CTF 2026 "rm -rf flag.png"

---

## Ext2 Orphaned Inode Recovery via fsck (BSidesSF 2026)

**Pattern (orphan):** A file has been deleted from an ext2 filesystem, leaving an orphaned inode. The file doesn't appear in any directory listing, but `fsck` detects the unattached inode and can reconnect it to `/lost+found`.

```bash
# Mount the image — no flag visible
sudo mount -o loop disk.img /mnt
ls /mnt  # Nothing useful

# Run fsck to detect orphaned inodes
sudo umount /mnt
e2fsck -y disk.img
# Output: "Unattached inode 13"
# Output: "Connect to /lost+found? yes"

# Re-mount and check lost+found
sudo mount -o loop disk.img /mnt
ls /mnt/lost+found/
# Found: #13
file /mnt/lost+found/\#13  # Identify file type (e.g., PNG)
cp /mnt/lost+found/\#13 recovered_flag.png
```

**Key insight:** Ext2/ext3/ext4 deletion removes directory entries but the inode and data blocks may persist until overwritten. `e2fsck` (with `-y` for auto-fix) detects these orphaned inodes and reconnects them to `/lost+found` with numeric names. For ext2 specifically (no journaling), recovery is more reliable because blocks aren't zeroed on deletion.

**When to recognize:** Challenge provides an ext2/ext3/ext4 filesystem image. Normal mounting shows nothing. Challenge hints at "deleted", "orphan", "lost", or "recovery". Always run `fsck` on forensics filesystem images.

**Alternative tools:**
- `debugfs` — interactive ext2 exploration: `debugfs disk.img` then `lsdel` to list deleted inodes
- `extundelete` — automated ext3/ext4 recovery
- `icat` (Sleuth Kit) — extract file by inode number: `icat disk.img 13 > recovered`

**References:** BSidesSF 2026 "orphan"

---

## Corrupted ZIP Repair via Header Field Manipulation (PlaidCTF 2017)

ZIP archives with corrupted filename length fields can be repaired by hex-editing both the Local File Header and Central Directory Entry.

```python
# ZIP Local File Header format (at offset 0x04 from PK\x03\x04):
# Offset 26: filename length (2 bytes, little-endian)
# ZIP Central Directory Entry (at PK\x01\x02):
# Offset 28: filename length (2 bytes, little-endian)

# Fix: set both filename lengths to actual filename size
import struct
with open('broken.zip', 'rb') as f:
    data = bytearray(f.read())

# Find and fix Local File Header filename length
lfh = data.index(b'PK\x03\x04')
struct.pack_into('<H', data, lfh + 26, 8)  # set to 8 bytes

# Find and fix Central Directory filename length
cde = data.index(b'PK\x01\x02')
struct.pack_into('<H', data, cde + 28, 8)  # must match

# Write fixed bytes as filename
data[lfh+30:lfh+38] = b'flag.txt'

with open('fixed.zip', 'wb') as f:
    f.write(data)

# Alternative: brute-force deflate at candidate offsets
import zlib
with open('broken.zip', 'rb') as f:
    raw = f.read()
for offset in range(0x1E, 0x100):
    try:
        result = zlib.decompress(raw[offset:], -15)
        print(f"Offset {offset:#x}: {result}")
        break
    except zlib.error:
        continue
```

**Key insight:** ZIP filename length fields appear in both the Local File Header (offset 26) and Central Directory (offset 28). Both must match and reflect the actual filename. When these are corrupted to absurd values (e.g., 9001), the archive appears empty. As a fallback, brute-force raw deflate decompression at candidate data offsets.

**Detection:** ZIP file that `unzip -l` reports as empty or produces errors about invalid filename lengths. `hexdump` shows valid `PK\x03\x04` and `PK\x01\x02` signatures but unreasonable values in length fields.

---

## Recovering Deleted .git Repository from FAT Image (Square CTF 2017)

A FAT filesystem image with a deleted `.git` directory. Use TSK `fls -r` to list all files including deleted ones (marked with `*`). Extract deleted inodes with `icat`. Reconstruct the git object directory structure from the extracted files, then use `git fsck` and `git log` to recover commit history and flag.

```bash
# Step 1: List all files including deleted ones (* prefix = deleted)
fls -r disk.img | grep '\*'
# Example output:
# r/r * 5:   .git/HEAD
# r/r * 6:   .git/config
# r/r * 7:   .git/objects/ab/cdef1234...

# Step 2: Extract deleted files by inode number
icat disk.img 5 > HEAD
icat disk.img 6 > config
# Repeat for all git object inodes

# Step 3: Rebuild .git directory structure
mkdir -p recovered/.git/objects/ab/
# Place each extracted object at its correct path

# Step 4: Recover commit history
cd recovered
git fsck --full        # Check object integrity, find dangling commits
git log --all          # Show all commits including unreferenced ones
git show <commit_hash> # Inspect specific commit for flag
```

**Key insight:** FAT marks deleted files by changing the first byte of the directory entry to `0xE5` but keeps cluster data intact until reused. TSK's `fls`/`icat` extracts deleted files by inode, making deletion forensically reversible. Git objects are content-addressed — once extracted, `git fsck` finds all reachable commits even without a valid HEAD reference.

---

## DNSSEC Key Recovery from Git Commit History (Hack.lu 2017)

DNSSEC private signing keys committed to a git repository and later deleted remain permanently in the commit history. Recover the keys to set up a local BIND instance and forge DNSSEC-signed DNS responses.

```bash
# Step 1: Find commits that deleted key files
git log --all --diff-filter=D -- '*.private' '*.key' 'Kexample.*.+*.+*.key'

# Step 2: Recover the deleted key files from the commit before deletion
git show <commit_hash>^:<path/to/Kzone.+005+12345.private> > recovered.private
git show <commit_hash>^:<path/to/Kzone.+005+12345.key> > recovered.key

# Alternative: search all commits for key material
git log --all -p -- '*.private' | grep -A 20 'Private-key-format'

# Step 3: Verify key contents
cat recovered.private
# Private-key-format: v1.3
# Algorithm: 5 (RSASHA1)
# ...

# Step 4: Use recovered keys to forge DNSSEC-signed responses
# Configure BIND with the recovered signing keys and sign the zone
dnssec-signzone -K /path/to/keys -o example.com zone.db
```

**Key insight:** Sensitive cryptographic key material in git history is permanently recoverable — `git log --diff-filter=D` finds all commits that deleted files, and `git show <commit>^:<path>` retrieves the file's state just before deletion. DNSSEC private keys enable forging any DNS record for the zone, allowing DNS cache poisoning or redirecting traffic to attacker-controlled servers.

---

## XZ Stream Header Repair via CRC32 Reconstruction (Hackover 2018)

**Pattern:** The file has a valid XZ stream footer but the stream header has been overwritten (commonly with `PK\x03\x04` to make it look like a ZIP). Rebuild the 12-byte XZ header from the format spec: magic `FD 37 7A 58 5A 00`, two bytes of stream flags, and a 4-byte little-endian CRC32 of those flags. Prepend the reconstructed header to the rest of the file and `xz -d` decompresses cleanly.

```bash
# 1. Confirm the footer — XZ stream footer magic is "YZ" at the end.
xxd broken.xz | tail -1
# 00002ff0: 00 00 01 59 5A  ...YZ

# 2. Read stream_flags from the footer (byte at offset -6 from EOF)
STREAM_FLAGS=$(xxd -p -s -6 -l 2 broken.xz)
# e.g. 00 04  → CHECK_CRC64

# 3. Compute CRC32 of the 2 flag bytes (little-endian output)
CRC=$(python3 -c "import binascii; print(binascii.crc32(bytes.fromhex('$STREAM_FLAGS')).to_bytes(4,'little').hex())")

# 4. Rebuild the header and replace the first 12 bytes
printf '\xFD7zXZ\x00' > newhdr.bin
printf '%s' "$STREAM_FLAGS" | xxd -r -p >> newhdr.bin
printf '%s' "$CRC"          | xxd -r -p >> newhdr.bin
dd if=newhdr.bin of=broken.xz bs=1 count=12 conv=notrunc

# 5. Decompress
xz -d broken.xz
```

**Key insight:** XZ streams are defined by a fixed 12-byte header and a 12-byte footer that both include the same `stream_flags` byte — when the header is damaged you can copy the flags out of the still-intact footer and recompute the header CRC32 locally. The same header-reconstruction trick works for any format where the checksum input is small enough to brute-force or derive from the footer: GZIP (trailing `isize`/`crc32`), ZIP (central directory before the local file header), and zstd (frame header with skip-frames). When the challenge hands you a blob whose magic bytes belong to the wrong format, check the **last few bytes** for the real footer signature before trying to salvage the header.

**References:** Hackover CTF 2018 — UnbreakMyStart, writeup 11508

---

## ZipCrypto Known-Plaintext Cracking via bkcrack (Codegate 2019)

**Pattern:** ZipCrypto (the legacy PKZIP stream cipher, not AES-256) falls to known-plaintext attacks when you have at least 12 bytes of known plaintext for an encrypted file. `pkcrack` is the classic tool but often fails on modern archives; `bkcrack` (https://github.com/kimci86/bkcrack) handles edge cases with partial headers.

```bash
# Extract any unencrypted neighbour and its encrypted version
unzip secret.zip unencrypted_known.txt
bkcrack -C secret.zip -c target.txt -p unencrypted_known.txt -P known.zip
# Decrypt the whole archive with the recovered internal state
bkcrack -C secret.zip -k <k0> <k1> <k2> -d target_decrypted.bin
```

**Key insight:** ZIP headers often include well-known constants (PNG/JPEG magic, empty `README.txt`, `.gitignore`). Any encrypted ZIP that also ships an unencrypted reference file — or where you can guess 12+ bytes of header — falls immediately to `bkcrack`. Swap to it when `pkcrack` throws.

**References:** Codegate CTF 2019 — Rich Project, writeup 12907

---

## SQLite Serial-Type Byte Forensics (RITSEC 2018)

**Pattern:** Two near-identical SQLite files differ only in selected bytes. SQLite records encode each column with a "serial type" varint that both describes the type and carries the length (types ≥13 mean strings, length `(type - 13) / 2`). Walk the records, locate the changed serial-type bytes between versions, and read the adjacent text payload to recover hidden characters.

```python
def extract_hidden(path):
    with open(path, 'rb') as f: db = f.read()
    offsets = [0x892, 0xBA5, 0xE13]   # diff the two files first
    return bytes(db[off] for off in offsets)
```

**Key insight:** SQLite's varint serial-type scheme stores metadata *inline* with the payload, so an attacker who can flip one varint changes the interpretation of the next N bytes. Diff two versions byte-by-byte, cluster the diffs by record, and decode each varint to locate hidden text fields.

**References:** RITSEC CTF 2018 — Lite Forensics, writeup 12223

---

## Recursive Binwalk Chain PNG->PDF->DOCX->PNG->Base64 (TAMUctf 2019)

**Pattern:** One carrier file hides a chain of embedded documents — PNG with a PDF appended, the PDF embeds a DOCX (which is a ZIP), the DOCX embeds another PNG, and that PNG has Base64 appended after the IEND/EOF. Each layer changes container format to evade naive string searches.

```bash
# Layer 1-2: carve everything out of the outer PNG (pulls PDF, ZIP streams, etc.)
binwalk --dd=".*" art.png
cd _art.png.extracted
file *                      # identify the Microsoft Word 2007+ blob

# Layer 3: DOCX is a ZIP archive
unzip 34591D -d docx/        # hex offset from binwalk becomes the filename
ls docx/word/media/          # image1.png is the next-layer carrier

# Layer 4: recurse binwalk into the inner PNG to pull an embedded PDF
binwalk --dd=".*" docx/word/media/image1.png

# Layer 5: check for data appended after %%EOF of the inner PDF
strings _image1.png.extracted/*.pdf | tail -n 10
# -> ZmxhZ3tQMGxZdEByX0QwX3kwdV9HM3RfSXRfTjB3P30K
echo 'ZmxhZ3tQMGxZdEByX0QwX3kwdV9HM3RfSXRfTjB3P30K' | base64 -d
```

**Key insight:** When `grep flag` on the outermost file fails, assume each extracted file is itself a carrier. DOCX/XLSX/PPTX/APK/JAR are all ZIPs, so `unzip` works directly. PDFs commonly carry data *after* the final `%%EOF`, so always `strings | tail` or seek past the trailer. `binwalk --dd=".*"` writes every signature hit to disk so you can recurse with minimal typing.

**References:** TAMUctf 2019 — I Heard You Like Files, writeups 13412 and 13587

---

## Regex-Password Nested Zip Chain with exrex (UTCTF 2019)

**Pattern:** Outer zip contains a `hint.txt` (regex) and `archive.zip`; the regex enumerates the password set for the inner zip. Each extracted zip produces the next regex hint. Chain is deep (1000+ layers) so it must be scripted. `exrex.generate(regex)` materialises every string matching a regex, which is perfect for constrained password spaces.

```python
import exrex, zipfile, os

hint = r'^  7  y  RU[A-Z]KKx2 R4\d[a-z]B  N$'
archive = 'RegularZips.zip'

for i in range(10000):
    candidates = list(exrex.generate(hint))
    out_dir = f'layer{i}'
    os.makedirs(out_dir, exist_ok=True)
    with zipfile.ZipFile(archive) as zf:
        for pw in candidates:
            try:
                zf.extractall(out_dir, pwd=pw.encode())
                print(f'[{i}] pw={pw}')
                break
            except Exception:
                continue
        else:
            raise RuntimeError(f'no password matched regex at layer {i}')
    with open(os.path.join(out_dir, 'hint.txt')) as f:
        hint = f.read().strip()
    archive = os.path.join(out_dir, 'archive.zip')
    if not os.path.exists(archive):
        print('FLAG IN', out_dir)
        break
```

**Key insight:** When a zip's password is described by a regex, don't brute ASCII — use `exrex` to enumerate only matching strings (often just a handful of candidates per layer). Automate the extract-read-hint-repeat cycle; 1000 layers finish in seconds because the search space per layer is tiny.

**References:** UTCTF 2019 — Regular Zips, writeups 13951 and 13861

---

## See Also

- [disk-and-memory.md](disk-and-memory.md) - Core disk/memory forensics (Volatility, disk image analysis, VM/OVA/VMDK, VMware snapshots, coredumps, KAPE triage, PowerShell ransomware, Android/Docker/cloud forensics, BSON reconstruction, TrueCrypt/VeraCrypt mounting)
- [disk-advanced.md](disk-advanced.md) - Advanced disk and memory techniques (deleted partitions, ZFS forensics, GPT GUID encoding, VMDK sparse parsing, memory dump string carving, ransomware key recovery, WordPerfect macro XOR, minidump ISO 9660 recovery, APFS snapshots, RAID 5 XOR recovery)
