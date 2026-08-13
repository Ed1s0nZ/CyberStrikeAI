# CTF Forensics - 3D Printing / CAD File Forensics

## Table of Contents
- [PrusaSlicer Binary G-code (.g / .bgcode)](#prusaslicer-binary-g-code-g--bgcode)
- [QOIF (Quite OK Image Format)](#qoif-quite-ok-image-format)
- [G-code Analysis Tips](#g-code-analysis-tips)
- [G-code Side View Visualization (0xFun 2026)](#g-code-side-view-visualization-0xfun-2026)
- [Uncommon File Magic Bytes](#uncommon-file-magic-bytes)

---

## PrusaSlicer Binary G-code (.g / .bgcode)

**File magic:** `GCDE` (4 bytes)

The `.g` extension is PrusaSlicer's binary G-code format (bgcode). It stores G-code in a block-based structure with compression.

**File structure:**
```text
Header: "GCDE"(4) + version(4) + checksum_type(2)
Blocks: [type(2) + compression(2) + uncompressed_size(4)
         + compressed_size(4) if compressed
         + type-specific fields
         + data + CRC32(4)]
```

**Block types:**
- 0 = FileMetadata (has encoding field, 2 bytes)
- 1 = GCode (has encoding field, 2 bytes)
- 2 = SlicerMetadata (has encoding field, 2 bytes)
- 3 = PrinterMetadata (has encoding field, 2 bytes)
- 4 = PrintMetadata (has encoding field, 2 bytes)
- 5 = Thumbnail (has format(2) + width(2) + height(2))

**Compression types:** 0=None, 1=Deflate, 2=Heatshrink(11,4), 3=Heatshrink(12,4)

**Thumbnail formats:** 0=PNG, 1=JPEG, 2=QOI (Quite OK Image)

**Parsing and extracting G-code:**
```python
import struct, zlib
import heatshrink2  # pip install heatshrink2

with open('file.g', 'rb') as f:
    data = f.read()

pos = 10  # After header
while pos < len(data) - 8:
    block_type = struct.unpack('<H', data[pos:pos+2])[0]
    compression = struct.unpack('<H', data[pos+2:pos+4])[0]
    uncompressed_size = struct.unpack('<I', data[pos+4:pos+8])[0]
    pos += 8
    if compression != 0:
        compressed_size = struct.unpack('<I', data[pos:pos+4])[0]
        pos += 4
    else:
        compressed_size = uncompressed_size
    # Type-specific extra header fields
    if block_type in [0,1,2,3,4]:
        pos += 2  # encoding field
    elif block_type == 5:
        pos += 6  # format + width + height
    block_data = data[pos:pos+compressed_size]
    pos += compressed_size + 4  # data + CRC32

    if block_type == 1:  # GCode block
        if compression == 3:  # Heatshrink 12/4
            gcode = heatshrink2.decompress(block_data, window_sz2=12, lookahead_sz2=4)
        elif compression == 1:  # Deflate (zlib)
            gcode = zlib.decompress(block_data)
        # Search gcode for hidden comments/flags
```

**Common hiding spots:**
- G-code comments (`;=== FLAG_CHAR ... ===`) at specific layer heights
- Custom G-code sections (`;TYPE:Custom`)
- Metadata fields (object names, filament info)
- Thumbnail images (extract and view QOIF/PNG)

## QOIF (Quite OK Image Format)

**Magic:** `qoif` (4 bytes) + width(4 BE) + height(4 BE) + channels(1) + colorspace(1)

Lightweight image format used in PrusaSlicer thumbnails. Decode with Python struct or use the `qoi` library.

## G-code Analysis Tips

```bash
# Search for flag patterns in decompressed gcode
grep -i "flag\|meta\|ctf\|secret" output.gcode

# Look for custom comments at layer changes
grep ";.*FLAG\|;.*LAYER_CHANGE" output.gcode

# Extract XY coordinates for visual patterns
grep "^G1" output.gcode | awk '{print $2, $3}' > coords.txt
```

## G-code Side View Visualization (0xFun 2026)

**Pattern (PrintedParts):** Plot X vs Z (side view) with Y filtering. Extrusion segments at specific Y ranges form readable text.

```bash
# Extract XY coordinates from G-code
grep "^G1" output.gcode | awk '{print $2, $3}' > coords.txt
# Plot with matplotlib for visual patterns
```

**Lesson:** G-code is just coordinate lists. Side projections (XZ or YZ) reveal embossed/engraved text.

---

## Uncommon File Magic Bytes

| Magic | Format | Extension | Notes |
|-------|--------|-----------|-------|
| `GCDE` | PrusaSlicer binary G-code | `.g`, `.bgcode` | 3D printing, heatshrink compressed |
| `qoif` | Quite OK Image Format | `.qoi` | Lightweight image format, often embedded |
| `OggS` | Ogg container | `.ogg` | Audio/video |
| `RIFF` | RIFF container | `.wav`,`.avi` | Check subformat |
| `%PDF` | PDF | `.pdf` | Check metadata & embedded objects |
