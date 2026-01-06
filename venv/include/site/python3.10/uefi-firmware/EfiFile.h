/** @file
  The firmware file related definitions in PI.

  Copyright (c) 2006 - 2013, Intel Corporation. All rights reserved.<BR>

  This program and the accompanying materials are licensed and made available
  under the terms and conditions of the BSD License which accompanies this
  distribution.  The full text of the license may be found at:
    http://opensource.org/licenses/bsd-license.php

  THE PROGRAM IS DISTRIBUTED UNDER THE BSD LICENSE ON AN "AS IS" BASIS,
  WITHOUT WARRANTIES OR REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.

  File Name: EfiFile.h
  Adapted from: PiFirmwareFile.h

  @par Revision Reference:
  Version 1.0.

**/

#ifndef __EFI_FILE_H__
#define __EFI_FILE_H__

#include "BaseTypes.h"

typedef UINT8 EFI_SECTION_TYPE;

typedef struct {
  UINT8             Size[3];
  EFI_SECTION_TYPE  Type;
} EFI_COMMON_SECTION_HEADER;

typedef struct {
  UINT8             Size[3];
  EFI_SECTION_TYPE  Type;
  UINT32            ExtendedSize;
} EFI_COMMON_SECTION_HEADER2;

#define MAX_SECTION_SIZE        0x1000000

//
// Leaf section type that contains an 
// IA-32 16-bit executable image.
// 
typedef EFI_COMMON_SECTION_HEADER EFI_COMPATIBILITY16_SECTION;
typedef EFI_COMMON_SECTION_HEADER2 EFI_COMPATIBILITY16_SECTION2;

//
// An encapsulation section type in which the 
// section data is compressed.
// 
typedef struct {
  EFI_COMMON_SECTION_HEADER   CommonHeader;
  UINT32                      UncompressedLength;
  UINT8                       CompressionType;
} EFI_COMPRESSION_SECTION;

typedef struct {
  EFI_COMMON_SECTION_HEADER2  CommonHeader;
  UINT32                      UncompressedLength;
  UINT8                       CompressionType;
} EFI_COMPRESSION_SECTION2;



#endif