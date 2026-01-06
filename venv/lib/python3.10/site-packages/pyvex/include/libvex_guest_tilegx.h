
/*---------------------------------------------------------------*/
/*--- begin                             libvex_guest_tilegx.h ---*/
/*---------------------------------------------------------------*/

/*
  This file is part of Valgrind, a dynamic binary instrumentation
  framework.

  Copyright (C) 2010-2015 Tilera Corp.

  This program is free software; you can redistribute it and/or
  modify it under the terms of the GNU General Public License as
  published by the Free Software Foundation; either version 2 of the
  License, or (at your option) any later version.

  This program is distributed in the hope that it will be useful, but
  WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
  General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with this program; if not, write to the Free Software
  Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA
  02111-1307, USA.

  The GNU General Public License is contained in the file COPYING.
*/

/* Contributed by Zhi-Gang Liu <zliu at tilera dot com> */

#ifndef __LIBVEX_PUB_GUEST_TILEGX_H
#define __LIBVEX_PUB_GUEST_TILEGX_H

#include "libvex_basictypes.h"
#include "libvex_emnote.h"

#undef   TILEGX_DEBUG

/*---------------------------------------------------------------*/
/*--- Vex's representation of the tilegx CPU state.           ---*/
/*---------------------------------------------------------------*/

typedef ULong ULONG;

typedef
struct {
  /* CPU Registers */
  /*   0   */ ULONG guest_r0;
  /*   8   */ ULONG guest_r1;
  /*   16  */ ULONG guest_r2;
  /*   24  */ ULONG guest_r3;
  /*   32  */ ULONG guest_r4;
  /*   40  */ ULONG guest_r5;
  /*   48  */ ULONG guest_r6;
  /*   56  */ ULONG guest_r7;
  /*   64  */ ULONG guest_r8;
  /*   72  */ ULONG guest_r9;
  /*   80  */ ULONG guest_r10;
  /*   88  */ ULONG guest_r11;
  /*   96  */ ULONG guest_r12;
  /*   104 */ ULONG guest_r13;
  /*   112 */ ULONG guest_r14;
  /*   120 */ ULONG guest_r15;
  /*   128 */ ULONG guest_r16;
  /*   136 */ ULONG guest_r17;
  /*   144 */ ULONG guest_r18;
  /*   152 */ ULONG guest_r19;
  /*   160 */ ULONG guest_r20;
  /*   168 */ ULONG guest_r21;
  /*   176 */ ULONG guest_r22;
  /*   184 */ ULONG guest_r23;
  /*   192 */ ULONG guest_r24;
  /*   200 */ ULONG guest_r25;
  /*   208 */ ULONG guest_r26;
  /*   216 */ ULONG guest_r27;
  /*   224 */ ULONG guest_r28;
  /*   232 */ ULONG guest_r29;
  /*   240 */ ULONG guest_r30;
  /*   248 */ ULONG guest_r31;
  /*   256 */ ULONG guest_r32;
  /*   264 */ ULONG guest_r33;
  /*   272 */ ULONG guest_r34;
  /*   280 */ ULONG guest_r35;
  /*   288 */ ULONG guest_r36;
  /*   296 */ ULONG guest_r37;
  /*   304 */ ULONG guest_r38;
  /*   312 */ ULONG guest_r39;
  /*   320 */ ULONG guest_r40;
  /*   328 */ ULONG guest_r41;
  /*   336 */ ULONG guest_r42;
  /*   344 */ ULONG guest_r43;
  /*   352 */ ULONG guest_r44;
  /*   360 */ ULONG guest_r45;
  /*   368 */ ULONG guest_r46;
  /*   376 */ ULONG guest_r47;
  /*   384 */ ULONG guest_r48;
  /*   392 */ ULONG guest_r49;
  /*   400 */ ULONG guest_r50;
  /*   408 */ ULONG guest_r51;
  /*   416 */ ULONG guest_r52; /* FP */
  /*   424 */ ULONG guest_r53;
  /*   432 */ ULONG guest_r54; /* SP */
  /*   440 */ ULONG guest_r55; /* LR */
  /*   448 */ ULONG guest_r56; /* zero */
  /*   456 */ ULONG guest_r57; /* Reserved */
  /*   464 */ ULONG guest_r58; /* Reserved */
  /*   472 */ ULONG guest_r59; /* Reserved */
  /*   480 */ ULONG guest_r60; /* Reserved */
  /*   488 */ ULONG guest_r61; /* Reserved */
  /*   496 */ ULONG guest_r62; /* Reserved */
  /*   504 */ ULONG guest_r63; /* Reserved */
  /*   512 */ ULONG guest_pc;
  /*   520 */ ULONG guest_spare; /* Reserved */
  /*   528 */ ULONG guest_EMNOTE;
  /*   536 */ ULONG guest_CMSTART;
  /*   544 */ ULONG guest_CMLEN;
  /*   552 */ ULONG guest_NRADDR;
  /*   560 */ ULong guest_cmpexch;
  /*   568 */ ULong guest_zero;
  /*   576 */ ULong guest_ex_context_0;
  /*   584 */ ULong guest_ex_context_1;
  /*   592 */ ULong host_EvC_FAILADDR;
  /*   600 */ ULong host_EvC_COUNTER;
  /*   608 */ ULong guest_COND;
  /*   616 */ ULong PAD;

} VexGuestTILEGXState;

#define OFFSET_tilegx_r(_N)  (8 * (_N))

/*---------------------------------------------------------------*/
/*--- Utility functions for TILEGX guest stuff.               ---*/
/*---------------------------------------------------------------*/

/* ALL THE FOLLOWING ARE VISIBLE TO LIBRARY CLIENT */

/* Initialise all guest TILEGX state. */

extern
void LibVEX_GuestTILEGX_initialise ( /*OUT*/VexGuestTILEGXState* vex_state );


#endif /* __LIBVEX_PUB_GUEST_TILEGX_H */


/*---------------------------------------------------------------*/
/*---                                   libvex_guest_tilegx.h ---*/
/*---------------------------------------------------------------*/
