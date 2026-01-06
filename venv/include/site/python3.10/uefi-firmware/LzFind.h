/* LzFind.h -- Match finder for LZ algorithms
2009-04-22 : Igor Pavlov : Public domain */

#ifndef __LZ_FIND_H
#define __LZ_FIND_H

#include "Types.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef SizeT CLzRef;

typedef struct _CMatchFinder
{
  Byte *buffer;
  SizeT pos;
  SizeT posLimit;
  SizeT streamPos;
  SizeT lenLimit;

  SizeT cyclicBufferPos;
  SizeT cyclicBufferSize; /* it must be = (historySize + 1) */

  SizeT matchMaxLen;
  CLzRef *hash;
  CLzRef *son;
  SizeT hashMask;
  SizeT cutValue;

  Byte *bufferBase;
  ISeqInStream *stream;
  ptrdiff_t streamEndWasReached;

  SizeT blockSize;
  SizeT keepSizeBefore;
  SizeT keepSizeAfter;

  SizeT numHashBytes;
  ptrdiff_t directInput;
  size_t directInputRem;
  ptrdiff_t btMode;
  ptrdiff_t bigHash;
  SizeT historySize;
  SizeT fixedHashSize;
  SizeT hashSizeSum;
  SizeT numSons;
  SRes result;
  SizeT crc[256];
} CMatchFinder;

#define Inline_MatchFinder_GetPointerToCurrentPos(p) ((p)->buffer)
#define Inline_MatchFinder_GetIndexByte(p, index) ((p)->buffer[(Int32)(index)])

#define Inline_MatchFinder_GetNumAvailableBytes(p) ((p)->streamPos - (p)->pos)

int MatchFinder_NeedMove(const CMatchFinder *p);
Byte *MatchFinder_GetPointerToCurrentPos(CMatchFinder *p);
void MatchFinder_MoveBlock(CMatchFinder *p);
void MatchFinder_ReadIfRequired(CMatchFinder *p);

void MatchFinder_Construct(CMatchFinder *p);

/* Conditions:
     historySize <= 3 GB
     keepAddBufferBefore + matchMaxLen + keepAddBufferAfter < 511MB
*/
int MatchFinder_Create(CMatchFinder *p, SizeT historySize,
    SizeT keepAddBufferBefore, SizeT matchMaxLen, SizeT keepAddBufferAfter,
    ISzAlloc *alloc);
void MatchFinder_Free(CMatchFinder *p, ISzAlloc *alloc);
void MatchFinder_Normalize3(SizeT subValue, CLzRef *items, SizeT numItems);
void MatchFinder_ReduceOffsets(CMatchFinder *p, SizeT subValue);

SizeT * GetMatchesSpec1(SizeT lenLimit, SizeT curMatch, SizeT pos, const Byte *buffer, CLzRef *son,
    SizeT _cyclicBufferPos, SizeT _cyclicBufferSize, SizeT _cutValue,
    SizeT *distances, SizeT maxLen);

/*
Conditions:
  Mf_GetNumAvailableBytes_Func must be called before each Mf_GetMatchLen_Func.
  Mf_GetPointerToCurrentPos_Func's result must be used only before any other function
*/

typedef void (*Mf_Init_Func)(void *object);
typedef Byte (*Mf_GetIndexByte_Func)(void *object, SizeT index);
typedef SizeT (*Mf_GetNumAvailableBytes_Func)(void *object);
typedef const Byte * (*Mf_GetPointerToCurrentPos_Func)(void *object);
typedef SizeT(*Mf_GetMatches_Func)(void *object, SizeT *distances);
typedef void (*Mf_Skip_Func)(void *object, SizeT);

typedef struct _IMatchFinder
{
  Mf_Init_Func Init;
  Mf_GetIndexByte_Func GetIndexByte;
  Mf_GetNumAvailableBytes_Func GetNumAvailableBytes;
  Mf_GetPointerToCurrentPos_Func GetPointerToCurrentPos;
  Mf_GetMatches_Func GetMatches;
  Mf_Skip_Func Skip;
} IMatchFinder;

void MatchFinder_CreateVTable(const CMatchFinder *p, IMatchFinder *vTable);

void MatchFinder_Init(CMatchFinder *p);
SizeT Bt3Zip_MatchFinder_GetMatches(CMatchFinder *p, SizeT *distances);
SizeT Hc3Zip_MatchFinder_GetMatches(CMatchFinder *p, SizeT *distances);
void Bt3Zip_MatchFinder_Skip(CMatchFinder *p, SizeT num);
void Hc3Zip_MatchFinder_Skip(CMatchFinder *p, SizeT num);

#ifdef __cplusplus
}
#endif

#endif
