
#ifndef __COMPRESSION_TYPES_H__
#define __COMPRESSION_TYPES_H__

#include "BaseTypes.h"

/*++

Routine Description:

  The compression routine.

Arguments:

  SrcBuffer   - The buffer storing the source data
  SrcSize     - The size of source data
  DstBuffer   - The buffer to store the compressed data
  DstSize     - On input, the size of DstBuffer; On output,
                the size of the actual compressed data.

Returns:

  EFI_BUFFER_TOO_SMALL  - The DstBuffer is too small. In this case,
                DstSize contains the size needed.
  EFI_SUCCESS           - Compression is successful.
  EFI_OUT_OF_RESOURCES  - No resource to complete function.
  EFI_INVALID_PARAMETER - Parameter supplied is wrong.

--*/
typedef
EFI_STATUS
(*COMPRESS_FUNCTION) (
  IN      char    *SrcBuffer,
  IN      size_t  SrcSize,
  IN      char    *DstBuffer,
  IN OUT  size_t  *DstSize
  );

typedef
EFI_STATUS
(*GETINFO_FUNCTION) (
  IN      VOID    *Source,
  IN      size_t  SrcSize,
  OUT     size_t  *DstSize,
  OUT     size_t  *ScratchSize
  );

typedef
EFI_STATUS
(*DECOMPRESS_FUNCTION) (
  IN      VOID    *Source,
  IN      size_t  SrcSize,
  IN OUT  VOID    *Destination,
  IN      size_t  DstSize,
  IN OUT  VOID    *Scratch,
  IN      size_t  ScratchSize
  );

#endif
