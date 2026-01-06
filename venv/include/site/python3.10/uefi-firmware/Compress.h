/** @file

Copyright (c) 2004 - 2008, Intel Corporation. All rights reserved.<BR>
This program and the accompanying materials                          
are licensed and made available under the terms and conditions of the BSD License         
which accompanies this distribution.  The full text of the license may be found at        
http://opensource.org/licenses/bsd-license.php                                            
                                                                                          
THE PROGRAM IS DISTRIBUTED UNDER THE BSD LICENSE ON AN "AS IS" BASIS,                     
WITHOUT WARRANTIES OR REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.             

Module Name:

  Compress.h

Abstract:

  Header file for compression routine.
  Providing both EFI and Tiano Compress algorithms.
  
**/

#ifndef _EFI_COMPRESS_H_
#define _EFI_COMPRESS_H_

#include <string.h>
#include <stdlib.h>

//#include "CommonLib.h"
#include "BaseTypes.h"

/*++

Routine Description:

  Tiano compression routine.

--*/
EFI_STATUS
TianoCompress (
  IN      UINT8   *SrcBuffer,
  IN      size_t  SrcSize,
  IN      UINT8   *DstBuffer,
  IN OUT  size_t  *DstSize
  )
;

/*++

Routine Description:

  Efi compression routine.

--*/
EFI_STATUS
EfiCompress (
  IN      UINT8   *SrcBuffer,
  IN      size_t  SrcSize,
  IN      UINT8   *DstBuffer,
  IN OUT  size_t  *DstSize
  )
;

#endif
