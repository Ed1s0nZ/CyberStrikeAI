/* LZMA Compress Header

  Copyright (c) 2012, Nikolaj Schlej. All rights reserved.
  This program and the accompanying materials
  are licensed and made available under the terms and conditions of the BSD License
  which accompanies this distribution.  The full text of the license may be found at
  http://opensource.org/licenses/bsd-license.php

  THE PROGRAM IS DISTRIBUTED UNDER THE BSD LICENSE ON AN "AS IS" BASIS,
  WITHWARRANTIES OR REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.

*/

#ifndef __LZMACOMPRESS_H__
#define __LZMACOMPRESS_H__

#include "SDK/C/Types.h"
#include "BaseTypes.h"

#define LZMA_DICTIONARY_SIZE 0x800000

EFI_STATUS
LzmaCompress (
  IN     UINT8   *Source,
  IN     SizeT  SourceSize,
  IN     UINT8   *Destination,
  IN OUT SizeT  *DestinationSize
  );

#endif