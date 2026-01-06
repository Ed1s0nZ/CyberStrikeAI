/** @file

Copyright (c) 2006 - 2008, Intel Corporation. All rights reserved.<BR>
This program and the accompanying materials                          
are licensed and made available under the terms and conditions of the BSD License         
which accompanies this distribution.  The full text of the license may be found at        
http://opensource.org/licenses/bsd-license.php                                            
                                                                                          
THE PROGRAM IS DISTRIBUTED UNDER THE BSD LICENSE ON AN "AS IS" BASIS,                     
WITHOUT WARRANTIES OR REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.             

Module Name:
  
  Decompress.h

Abstract:

  Header file for compression routine
  
**/

#ifndef _EFI_DECOMPRESS_H
#define _EFI_DECOMPRESS_H

#include "BaseTypes.h"

EFI_STATUS
EfiGetInfo (
  IN      VOID    *Source,
  IN      size_t  SrcSize,
  OUT     size_t  *DstSize,
  OUT     size_t  *ScratchSize
  );
/**

Routine Description:

  The implementation Efi Decompress GetInfo().

Arguments:

  Source      - The source buffer containing the compressed data.
  SrcSize     - The size of source buffer
  DstSize     - The size of destination buffer.
  ScratchSize - The size of scratch buffer.

Returns:

  EFI_SUCCESS           - The size of destination buffer and the size of scratch buffer are successull retrieved.
  EFI_INVALID_PARAMETER - The source data is corrupted

**/

EFI_STATUS
EfiDecompress (
  IN      VOID    *Source,
  IN      size_t  SrcSize,
  IN OUT  VOID    *Destination,
  IN      size_t  DstSize,
  IN OUT  VOID    *Scratch,
  IN      size_t  ScratchSize
  );
/**

Routine Description:

  The implementation of Efi Decompress().

Arguments:

  Source      - The source buffer containing the compressed data.
  SrcSize     - The size of source buffer
  Destination - The destination buffer to store the decompressed data
  DstSize     - The size of destination buffer.
  Scratch     - The buffer used internally by the decompress routine. This  buffer is needed to store intermediate data.
  ScratchSize - The size of scratch buffer.

Returns:

  EFI_SUCCESS           - Decompression is successfull
  EFI_INVALID_PARAMETER - The source data is corrupted

**/

EFI_STATUS
TianoGetInfo (
  IN      VOID    *Source,
  IN      size_t   SrcSize,
  OUT     size_t   *DstSize,
  OUT     size_t   *ScratchSize
  );
/**

Routine Description:

  The implementation Tiano Decompress GetInfo().

Arguments:

  Source      - The source buffer containing the compressed data.
  SrcSize     - The size of source buffer
  DstSize     - The size of destination buffer.
  ScratchSize - The size of scratch buffer.

Returns:

  EFI_SUCCESS           - The size of destination buffer and the size of scratch buffer are successull retrieved.
  EFI_INVALID_PARAMETER - The source data is corrupted

**/

EFI_STATUS
TianoDecompress (
  IN      VOID    *Source,
  IN      size_t  SrcSize,
  IN OUT  VOID    *Destination,
  IN      size_t  DstSize,
  IN OUT  VOID    *Scratch,
  IN      size_t  ScratchSize
  );
/**

Routine Description:

  The implementation of Tiano Decompress().

Arguments:

  Source      - The source buffer containing the compressed data.
  SrcSize     - The size of source buffer
  Destination - The destination buffer to store the decompressed data
  DstSize     - The size of destination buffer.
  Scratch     - The buffer used internally by the decompress routine. This  buffer is needed to store intermediate data.
  ScratchSize - The size of scratch buffer.

Returns:

  EFI_SUCCESS           - Decompression is successfull
  EFI_INVALID_PARAMETER - The source data is corrupted

**/

#endif
