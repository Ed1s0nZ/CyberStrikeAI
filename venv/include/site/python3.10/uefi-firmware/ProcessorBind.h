/* Processor or Compiler specific defines and types.

Copyright (c) 2006 - 2008, Intel Corporation. All rights reserved.

This program and the accompanying materials are licensed and made available
under the terms and conditions of the BSD License which accompanies this
distribution.  The full text of the license may be found at:
  http://opensource.org/licenses/bsd-license.php

THE PROGRAM IS DISTRIBUTED UNDER THE BSD LICENSE ON AN "AS IS" BASIS,
WITHOUT WARRANTIES OR REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.

File Name:  ProcessorBind.h

Modified for uefi_firmware_parser: 
This BaseTypes includes structures from various edk2 header files.

*/

#ifndef __PROCESSOR_BIND_H__
#define __PROCESSOR_BIND_H__

//
// Make sure we are useing the correct packing rules
//
#ifndef __GNUC__
#pragma pack()
#endif

#if _MSC_EXTENSIONS

//
// Disable warning that make it impossible to compile at /W4
// This only works for Microsoft* tools
//

//
// Disabling the unreferenced formal parameter warnings.
//
#pragma warning ( disable : 4100 )

#endif


#include "stdint.h"
typedef uint8_t   BOOLEAN;
typedef int8_t    INT8;
typedef uint8_t   UINT8;
typedef int16_t   INT16;
typedef uint16_t  UINT16;
typedef int32_t   INT32;
typedef uint32_t  UINT32;
typedef int64_t   INT64;
typedef uint64_t  UINT64;
typedef char      CHAR8;
typedef uint16_t  CHAR16;

typedef INT32     INTN;
typedef UINT32    UINTN;

#if _MSC_EXTENSIONS
  //
  // Microsoft* compiler requires _EFIAPI useage, __cdecl is Microsoft* specific C.
  // 
  #define EFIAPI __cdecl  
#endif

#if __GNUC__
  #define EFIAPI __attribute__((cdecl))    
#endif

//
// The Microsoft* C compiler can removed references to unreferenced data items
//  if the /OPT:REF linker option is used. We defined a macro as this is a 
//  a non standard extension
//
#if _MSC_EXTENSIONS
  #define GLOBAL_REMOVE_IF_UNREFERENCED __declspec(selectany)
#else
  #define GLOBAL_REMOVE_IF_UNREFERENCED
#endif

#endif
