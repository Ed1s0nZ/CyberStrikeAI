/*  LzmaEnc.h -- LZMA Encoder
2009-02-07 : Igor Pavlov : Public domain */

#ifndef __LZMA_ENC_H
#define __LZMA_ENC_H

#include "Types.h"

#ifdef __cplusplus
extern "C" {
#endif

#define LZMA_PROPS_SIZE 5

typedef struct _CLzmaEncProps
{
  int level;              /* 0 <= level <= 9 */
  SizeT dictSize;         /* (1 << 12) <= dictSize <= (1 << 27) for 32-bit version
                             (1 << 12) <= dictSize <= (1 << 30) for 64-bit version
                             default = (1 << 24) */
  ptrdiff_t lc;           /* 0 <= lc <= 8, default = 3 */
  ptrdiff_t lp;           /* 0 <= lp <= 4, default = 0 */
  ptrdiff_t pb;           /* 0 <= pb <= 4, default = 2 */
  ptrdiff_t algo;         /* 0 - fast, 1 - normal, default = 1 */
  ptrdiff_t fb;           /* 5 <= fb <= 273, default = 32 */
  ptrdiff_t btMode;       /* 0 - hashChain Mode, 1 - binTree mode - normal, default = 1 */
  ptrdiff_t numHashBytes; /* 2, 3 or 4, default = 4 */
  SizeT  mc;              /* 1 <= mc <= (1 << 30), default = 32 */
  SizeT writeEndMark;     /* 0 - do not write EOPM, 1 - write EOPM, default = 0 */
  ptrdiff_t numThreads;   /* 1 or 2, default = 2 */
} CLzmaEncProps;

void LzmaEncProps_Init(CLzmaEncProps *p);
void LzmaEncProps_Normalize(CLzmaEncProps *p);
SizeT LzmaEncProps_GetDictSize(const CLzmaEncProps *props2);


/* ---------- CLzmaEncHandle Interface ---------- */

/* LzmaEnc_* functions can return the following exit codes:
Returns:
  SZ_OK           - OK
  SZ_ERROR_MEM    - Memory allocation error
  SZ_ERROR_PARAM  - Incorrect paramater in props
  SZ_ERROR_WRITE  - Write callback error.
  SZ_ERROR_PROGRESS - some break from progress callback
  SZ_ERROR_THREAD - errors in multithreading functions (only for Mt version)
*/

typedef void * CLzmaEncHandle;

CLzmaEncHandle LzmaEnc_Create(ISzAlloc *alloc);
void LzmaEnc_Destroy(CLzmaEncHandle p, ISzAlloc *alloc, ISzAlloc *allocBig);
SRes LzmaEnc_SetProps(CLzmaEncHandle p, const CLzmaEncProps *props);
SRes LzmaEnc_WriteProperties(CLzmaEncHandle p, Byte *properties, SizeT *size);
SRes LzmaEnc_Encode(CLzmaEncHandle p, ISeqOutStream *outStream, ISeqInStream *inStream,
    ICompressProgress *progress, ISzAlloc *alloc, ISzAlloc *allocBig);
SRes LzmaEnc_MemEncode(CLzmaEncHandle p, Byte *dest, SizeT *destLen, const Byte *src, SizeT srcLen,
    ptrdiff_t writeEndMark, ICompressProgress *progress, ISzAlloc *alloc, ISzAlloc *allocBig);

/* ---------- One Call Interface ---------- */

/* LzmaEncode
Return code:
  SZ_OK               - OK
  SZ_ERROR_MEM        - Memory allocation error
  SZ_ERROR_PARAM      - Incorrect paramater
  SZ_ERROR_OUTPUT_EOF - output buffer overflow
  SZ_ERROR_THREAD     - errors in multithreading functions (only for Mt version)
*/

SRes LzmaEncode(Byte *dest, SizeT *destLen, const Byte *src, SizeT srcLen,
    const CLzmaEncProps *props, Byte *propsEncoded, SizeT *propsSize, ptrdiff_t writeEndMark,
    ICompressProgress *progress, ISzAlloc *alloc, ISzAlloc *allocBig);

#ifdef __cplusplus
}
#endif

#endif
