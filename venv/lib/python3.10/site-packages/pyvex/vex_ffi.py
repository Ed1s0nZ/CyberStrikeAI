ffi_str = """typedef unsigned char UChar;
typedef signed char Char;
typedef char HChar;
typedef unsigned short UShort;
typedef signed short Short;
typedef unsigned int UInt;
typedef signed int Int;
typedef unsigned long long int ULong;
typedef signed long long int Long;
typedef unsigned long SizeT;
typedef UInt U128[4];
typedef UInt U256[8];
typedef
   union {
      UChar w8[16];
      UShort w16[8];
      UInt w32[4];
      ULong w64[2];
   }
   V128;
typedef
   union {
      UChar w8[32];
      UShort w16[16];
      UInt w32[8];
      ULong w64[4];
   }
   V256;
typedef float Float;
typedef double Double;
typedef unsigned char Bool;
typedef UInt Addr32;
typedef ULong Addr64;
typedef Addr64 Addr;
typedef unsigned long HWord;
typedef
   enum {
      Ity_INVALID=0x1100,
      Ity_I1,
      Ity_I8,
      Ity_I16,
      Ity_I32,
      Ity_I64,
      Ity_I128,
      Ity_F16,
      Ity_F32,
      Ity_F64,
      Ity_D32,
      Ity_D64,
      Ity_D128,
      Ity_F128,
      Ity_V128,
      Ity_V256
   }
   IRType;
extern void ppIRType ( IRType );
extern Int sizeofIRType ( IRType );
extern IRType integerIRTypeOfSize ( Int szB );
typedef
   enum {
      Iend_LE=0x1200,
      Iend_BE
   }
   IREndness;
typedef
   enum {
      Ico_U1=0x1300,
      Ico_U8,
      Ico_U16,
      Ico_U32,
      Ico_U64,
      Ico_F32,
      Ico_F32i,
      Ico_F64,
      Ico_F64i,
      Ico_V128,
      Ico_V256
   }
   IRConstTag;
typedef
   struct _IRConst {
      IRConstTag tag;
      union {
         Bool U1;
         UChar U8;
         UShort U16;
         UInt U32;
         ULong U64;
         Float F32;
         UInt F32i;
         Double F64;
         ULong F64i;
         UShort V128;
         UInt V256;
      } Ico;
   }
   IRConst;
extern IRConst* IRConst_U1 ( Bool );
extern IRConst* IRConst_U8 ( UChar );
extern IRConst* IRConst_U16 ( UShort );
extern IRConst* IRConst_U32 ( UInt );
extern IRConst* IRConst_U64 ( ULong );
extern IRConst* IRConst_F32 ( Float );
extern IRConst* IRConst_F32i ( UInt );
extern IRConst* IRConst_F64 ( Double );
extern IRConst* IRConst_F64i ( ULong );
extern IRConst* IRConst_V128 ( UShort );
extern IRConst* IRConst_V256 ( UInt );
extern IRConst* deepCopyIRConst ( const IRConst* );
extern void ppIRConst ( const IRConst* );
extern Bool eqIRConst ( const IRConst*, const IRConst* );
typedef
   struct {
      Int regparms;
      const HChar* name;
      void* addr;
      UInt mcx_mask;
   }
   IRCallee;
extern IRCallee* mkIRCallee ( Int regparms, const HChar* name, void* addr );
extern IRCallee* deepCopyIRCallee ( const IRCallee* );
extern void ppIRCallee ( const IRCallee* );
typedef
   struct {
      Int base;
      IRType elemTy;
      Int nElems;
   }
   IRRegArray;
extern IRRegArray* mkIRRegArray ( Int, IRType, Int );
extern IRRegArray* deepCopyIRRegArray ( const IRRegArray* );
extern void ppIRRegArray ( const IRRegArray* );
extern Bool eqIRRegArray ( const IRRegArray*, const IRRegArray* );
typedef UInt IRTemp;
extern void ppIRTemp ( IRTemp );
typedef
   enum {
      Iop_INVALID=0x1400,
      Iop_Add8, Iop_Add16, Iop_Add32, Iop_Add64,
      Iop_Sub8, Iop_Sub16, Iop_Sub32, Iop_Sub64,
      Iop_Mul8, Iop_Mul16, Iop_Mul32, Iop_Mul64,
      Iop_Or8, Iop_Or16, Iop_Or32, Iop_Or64,
      Iop_And8, Iop_And16, Iop_And32, Iop_And64,
      Iop_Xor8, Iop_Xor16, Iop_Xor32, Iop_Xor64,
      Iop_Shl8, Iop_Shl16, Iop_Shl32, Iop_Shl64,
      Iop_Shr8, Iop_Shr16, Iop_Shr32, Iop_Shr64,
      Iop_Sar8, Iop_Sar16, Iop_Sar32, Iop_Sar64,
      Iop_CmpEQ8, Iop_CmpEQ16, Iop_CmpEQ32, Iop_CmpEQ64,
      Iop_CmpNE8, Iop_CmpNE16, Iop_CmpNE32, Iop_CmpNE64,
      Iop_Not8, Iop_Not16, Iop_Not32, Iop_Not64,
      Iop_CasCmpEQ8, Iop_CasCmpEQ16, Iop_CasCmpEQ32, Iop_CasCmpEQ64,
      Iop_CasCmpNE8, Iop_CasCmpNE16, Iop_CasCmpNE32, Iop_CasCmpNE64,
      Iop_ExpCmpNE8, Iop_ExpCmpNE16, Iop_ExpCmpNE32, Iop_ExpCmpNE64,
      Iop_MullS8, Iop_MullS16, Iop_MullS32, Iop_MullS64,
      Iop_MullU8, Iop_MullU16, Iop_MullU32, Iop_MullU64,
      Iop_Clz64, Iop_Clz32,
      Iop_Ctz64, Iop_Ctz32,
      Iop_CmpLT32S, Iop_CmpLT64S,
      Iop_CmpLE32S, Iop_CmpLE64S,
      Iop_CmpLT32U, Iop_CmpLT64U,
      Iop_CmpLE32U, Iop_CmpLE64U,
      Iop_CmpNEZ8, Iop_CmpNEZ16, Iop_CmpNEZ32, Iop_CmpNEZ64,
      Iop_CmpwNEZ32, Iop_CmpwNEZ64,
      Iop_Left8, Iop_Left16, Iop_Left32, Iop_Left64,
      Iop_Max32U,
      Iop_CmpORD32U, Iop_CmpORD64U,
      Iop_CmpORD32S, Iop_CmpORD64S,
      Iop_DivU32,
      Iop_DivS32,
      Iop_DivU64,
      Iop_DivS64,
      Iop_DivU64E,
      Iop_DivS64E,
      Iop_DivU32E,
      Iop_DivS32E,
      Iop_DivModU64to32,
      Iop_DivModS64to32,
      Iop_DivModU128to64,
      Iop_DivModS128to64,
      Iop_DivModS64to64,
      Iop_8Uto16, Iop_8Uto32, Iop_8Uto64,
                  Iop_16Uto32, Iop_16Uto64,
                               Iop_32Uto64,
      Iop_8Sto16, Iop_8Sto32, Iop_8Sto64,
                  Iop_16Sto32, Iop_16Sto64,
                               Iop_32Sto64,
      Iop_64to8, Iop_32to8, Iop_64to16,
      Iop_16to8,
      Iop_16HIto8,
      Iop_8HLto16,
      Iop_32to16,
      Iop_32HIto16,
      Iop_16HLto32,
      Iop_64to32,
      Iop_64HIto32,
      Iop_32HLto64,
      Iop_128to64,
      Iop_128HIto64,
      Iop_64HLto128,
      Iop_Not1,
      Iop_32to1,
      Iop_64to1,
      Iop_1Uto8,
      Iop_1Uto32,
      Iop_1Uto64,
      Iop_1Sto8,
      Iop_1Sto16,
      Iop_1Sto32,
      Iop_1Sto64,
      Iop_AddF64, Iop_SubF64, Iop_MulF64, Iop_DivF64,
      Iop_AddF32, Iop_SubF32, Iop_MulF32, Iop_DivF32,
      Iop_AddF64r32, Iop_SubF64r32, Iop_MulF64r32, Iop_DivF64r32,
      Iop_NegF64, Iop_AbsF64,
      Iop_NegF32, Iop_AbsF32,
      Iop_SqrtF64,
      Iop_SqrtF32,
      Iop_CmpF64,
      Iop_CmpF32,
      Iop_CmpF128,
      Iop_F64toI16S,
      Iop_F64toI32S,
      Iop_F64toI64S,
      Iop_F64toI64U,
      Iop_F64toI32U,
      Iop_I32StoF64,
      Iop_I64StoF64,
      Iop_I64UtoF64,
      Iop_I64UtoF32,
      Iop_I32UtoF32,
      Iop_I32UtoF64,
      Iop_F32toI32S,
      Iop_F32toI64S,
      Iop_F32toI32U,
      Iop_F32toI64U,
      Iop_I32StoF32,
      Iop_I64StoF32,
      Iop_F32toF64,
      Iop_F64toF32,
      Iop_ReinterpF64asI64, Iop_ReinterpI64asF64,
      Iop_ReinterpF32asI32, Iop_ReinterpI32asF32,
      Iop_F64HLtoF128,
      Iop_F128HItoF64,
      Iop_F128LOtoF64,
      Iop_AddF128, Iop_SubF128, Iop_MulF128, Iop_DivF128,
      Iop_MAddF128,
      Iop_MSubF128,
      Iop_NegMAddF128,
      Iop_NegMSubF128,
      Iop_NegF128, Iop_AbsF128,
      Iop_SqrtF128,
      Iop_I32StoF128,
      Iop_I64StoF128,
      Iop_I32UtoF128,
      Iop_I64UtoF128,
      Iop_F32toF128,
      Iop_F64toF128,
      Iop_F128toI32S,
      Iop_F128toI64S,
      Iop_F128toI32U,
      Iop_F128toI64U,
      Iop_F128toI128S,
      Iop_F128toF64,
      Iop_F128toF32,
      Iop_RndF128,
      Iop_TruncF128toI32S,
      Iop_TruncF128toI32U,
      Iop_TruncF128toI64U,
      Iop_TruncF128toI64S,
      Iop_AtanF64,
      Iop_Yl2xF64,
      Iop_Yl2xp1F64,
      Iop_PRemF64,
      Iop_PRemC3210F64,
      Iop_PRem1F64,
      Iop_PRem1C3210F64,
      Iop_ScaleF64,
      Iop_SinF64,
      Iop_CosF64,
      Iop_TanF64,
      Iop_2xm1F64,
      Iop_RoundF128toInt,
      Iop_RoundF64toInt,
      Iop_RoundF32toInt,
      Iop_MAddF32, Iop_MSubF32,
      Iop_MAddF64, Iop_MSubF64,
      Iop_MAddF64r32, Iop_MSubF64r32,
      Iop_RSqrtEst5GoodF64,
      Iop_RoundF64toF64_NEAREST,
      Iop_RoundF64toF64_NegINF,
      Iop_RoundF64toF64_PosINF,
      Iop_RoundF64toF64_ZERO,
      Iop_TruncF64asF32,
      Iop_RoundF64toF32,
      Iop_RecpExpF64,
      Iop_RecpExpF32,
      Iop_MaxNumF64,
      Iop_MinNumF64,
      Iop_MaxNumF32,
      Iop_MinNumF32,
      Iop_F16toF64,
      Iop_F64toF16,
      Iop_F16toF32,
      Iop_F32toF16,
      Iop_QAdd32S,
      Iop_QSub32S,
      Iop_Add16x2, Iop_Sub16x2,
      Iop_QAdd16Sx2, Iop_QAdd16Ux2,
      Iop_QSub16Sx2, Iop_QSub16Ux2,
      Iop_HAdd16Ux2, Iop_HAdd16Sx2,
      Iop_HSub16Ux2, Iop_HSub16Sx2,
      Iop_Add8x4, Iop_Sub8x4,
      Iop_QAdd8Sx4, Iop_QAdd8Ux4,
      Iop_QSub8Sx4, Iop_QSub8Ux4,
      Iop_HAdd8Ux4, Iop_HAdd8Sx4,
      Iop_HSub8Ux4, Iop_HSub8Sx4,
      Iop_Sad8Ux4,
      Iop_CmpNEZ16x2, Iop_CmpNEZ8x4,
      Iop_I32UtoFx2, Iop_I32StoFx2,
      Iop_FtoI32Ux2_RZ, Iop_FtoI32Sx2_RZ,
      Iop_F32ToFixed32Ux2_RZ, Iop_F32ToFixed32Sx2_RZ,
      Iop_Fixed32UToF32x2_RN, Iop_Fixed32SToF32x2_RN,
      Iop_Max32Fx2, Iop_Min32Fx2,
      Iop_PwMax32Fx2, Iop_PwMin32Fx2,
      Iop_CmpEQ32Fx2, Iop_CmpGT32Fx2, Iop_CmpGE32Fx2,
      Iop_RecipEst32Fx2,
      Iop_RecipStep32Fx2,
      Iop_RSqrtEst32Fx2,
      Iop_RSqrtStep32Fx2,
      Iop_Neg32Fx2, Iop_Abs32Fx2,
      Iop_CmpNEZ8x8, Iop_CmpNEZ16x4, Iop_CmpNEZ32x2,
      Iop_Add8x8, Iop_Add16x4, Iop_Add32x2,
      Iop_QAdd8Ux8, Iop_QAdd16Ux4, Iop_QAdd32Ux2, Iop_QAdd64Ux1,
      Iop_QAdd8Sx8, Iop_QAdd16Sx4, Iop_QAdd32Sx2, Iop_QAdd64Sx1,
      Iop_PwAdd8x8, Iop_PwAdd16x4, Iop_PwAdd32x2,
      Iop_PwMax8Sx8, Iop_PwMax16Sx4, Iop_PwMax32Sx2,
      Iop_PwMax8Ux8, Iop_PwMax16Ux4, Iop_PwMax32Ux2,
      Iop_PwMin8Sx8, Iop_PwMin16Sx4, Iop_PwMin32Sx2,
      Iop_PwMin8Ux8, Iop_PwMin16Ux4, Iop_PwMin32Ux2,
      Iop_PwAddL8Ux8, Iop_PwAddL16Ux4, Iop_PwAddL32Ux2,
      Iop_PwAddL8Sx8, Iop_PwAddL16Sx4, Iop_PwAddL32Sx2,
      Iop_Sub8x8, Iop_Sub16x4, Iop_Sub32x2,
      Iop_QSub8Ux8, Iop_QSub16Ux4, Iop_QSub32Ux2, Iop_QSub64Ux1,
      Iop_QSub8Sx8, Iop_QSub16Sx4, Iop_QSub32Sx2, Iop_QSub64Sx1,
      Iop_Abs8x8, Iop_Abs16x4, Iop_Abs32x2,
      Iop_Mul8x8, Iop_Mul16x4, Iop_Mul32x2,
      Iop_Mul32Fx2,
      Iop_MulHi16Ux4,
      Iop_MulHi16Sx4,
      Iop_PolynomialMul8x8,
      Iop_QDMulHi16Sx4, Iop_QDMulHi32Sx2,
      Iop_QRDMulHi16Sx4, Iop_QRDMulHi32Sx2,
      Iop_Avg8Ux8,
      Iop_Avg16Ux4,
      Iop_Max8Sx8, Iop_Max16Sx4, Iop_Max32Sx2,
      Iop_Max8Ux8, Iop_Max16Ux4, Iop_Max32Ux2,
      Iop_Min8Sx8, Iop_Min16Sx4, Iop_Min32Sx2,
      Iop_Min8Ux8, Iop_Min16Ux4, Iop_Min32Ux2,
      Iop_CmpEQ8x8, Iop_CmpEQ16x4, Iop_CmpEQ32x2,
      Iop_CmpGT8Ux8, Iop_CmpGT16Ux4, Iop_CmpGT32Ux2,
      Iop_CmpGT8Sx8, Iop_CmpGT16Sx4, Iop_CmpGT32Sx2,
      Iop_Cnt8x8,
      Iop_Clz8x8, Iop_Clz16x4, Iop_Clz32x2,
      Iop_Cls8x8, Iop_Cls16x4, Iop_Cls32x2,
      Iop_Clz64x2,
      Iop_Ctz8x16, Iop_Ctz16x8, Iop_Ctz32x4, Iop_Ctz64x2,
      Iop_Shl8x8, Iop_Shl16x4, Iop_Shl32x2,
      Iop_Shr8x8, Iop_Shr16x4, Iop_Shr32x2,
      Iop_Sar8x8, Iop_Sar16x4, Iop_Sar32x2,
      Iop_Sal8x8, Iop_Sal16x4, Iop_Sal32x2, Iop_Sal64x1,
      Iop_ShlN8x8, Iop_ShlN16x4, Iop_ShlN32x2,
      Iop_ShrN8x8, Iop_ShrN16x4, Iop_ShrN32x2,
      Iop_SarN8x8, Iop_SarN16x4, Iop_SarN32x2,
      Iop_QShl8x8, Iop_QShl16x4, Iop_QShl32x2, Iop_QShl64x1,
      Iop_QSal8x8, Iop_QSal16x4, Iop_QSal32x2, Iop_QSal64x1,
      Iop_QShlNsatSU8x8, Iop_QShlNsatSU16x4,
      Iop_QShlNsatSU32x2, Iop_QShlNsatSU64x1,
      Iop_QShlNsatUU8x8, Iop_QShlNsatUU16x4,
      Iop_QShlNsatUU32x2, Iop_QShlNsatUU64x1,
      Iop_QShlNsatSS8x8, Iop_QShlNsatSS16x4,
      Iop_QShlNsatSS32x2, Iop_QShlNsatSS64x1,
      Iop_QNarrowBin16Sto8Ux8,
      Iop_QNarrowBin16Sto8Sx8, Iop_QNarrowBin32Sto16Sx4,
      Iop_NarrowBin16to8x8, Iop_NarrowBin32to16x4,
      Iop_InterleaveHI8x8, Iop_InterleaveHI16x4, Iop_InterleaveHI32x2,
      Iop_InterleaveLO8x8, Iop_InterleaveLO16x4, Iop_InterleaveLO32x2,
      Iop_InterleaveOddLanes8x8, Iop_InterleaveEvenLanes8x8,
      Iop_InterleaveOddLanes16x4, Iop_InterleaveEvenLanes16x4,
      Iop_CatOddLanes8x8, Iop_CatOddLanes16x4,
      Iop_CatEvenLanes8x8, Iop_CatEvenLanes16x4,
      Iop_GetElem8x8, Iop_GetElem16x4, Iop_GetElem32x2,
      Iop_SetElem8x8, Iop_SetElem16x4, Iop_SetElem32x2,
      Iop_Dup8x8, Iop_Dup16x4, Iop_Dup32x2,
      Iop_Slice64,
      Iop_Reverse8sIn16_x4,
      Iop_Reverse8sIn32_x2, Iop_Reverse16sIn32_x2,
      Iop_Reverse8sIn64_x1, Iop_Reverse16sIn64_x1, Iop_Reverse32sIn64_x1,
      Iop_Perm8x8,
      Iop_GetMSBs8x8,
      Iop_RecipEst32Ux2, Iop_RSqrtEst32Ux2,
      Iop_AddD64, Iop_SubD64, Iop_MulD64, Iop_DivD64,
      Iop_AddD128, Iop_SubD128, Iop_MulD128, Iop_DivD128,
      Iop_ShlD64, Iop_ShrD64,
      Iop_ShlD128, Iop_ShrD128,
      Iop_D32toD64,
      Iop_D64toD128,
      Iop_I32StoD128,
      Iop_I32UtoD128,
      Iop_I64StoD128,
      Iop_I64UtoD128,
      Iop_D64toD32,
      Iop_D128toD64,
      Iop_I32StoD64,
      Iop_I32UtoD64,
      Iop_I64StoD64,
      Iop_I64UtoD64,
      Iop_D64toI32S,
      Iop_D64toI32U,
      Iop_D64toI64S,
      Iop_D64toI64U,
      Iop_D128toI32S,
      Iop_D128toI32U,
      Iop_D128toI64S,
      Iop_D128toI64U,
      Iop_F32toD32,
      Iop_F32toD64,
      Iop_F32toD128,
      Iop_F64toD32,
      Iop_F64toD64,
      Iop_F64toD128,
      Iop_F128toD32,
      Iop_F128toD64,
      Iop_F128toD128,
      Iop_D32toF32,
      Iop_D32toF64,
      Iop_D32toF128,
      Iop_D64toF32,
      Iop_D64toF64,
      Iop_D64toF128,
      Iop_D128toF32,
      Iop_D128toF64,
      Iop_D128toF128,
      Iop_RoundD64toInt,
      Iop_RoundD128toInt,
      Iop_CmpD64,
      Iop_CmpD128,
      Iop_CmpExpD64,
      Iop_CmpExpD128,
      Iop_QuantizeD64,
      Iop_QuantizeD128,
      Iop_SignificanceRoundD64,
      Iop_SignificanceRoundD128,
      Iop_ExtractExpD64,
      Iop_ExtractExpD128,
      Iop_ExtractSigD64,
      Iop_ExtractSigD128,
      Iop_InsertExpD64,
      Iop_InsertExpD128,
      Iop_D64HLtoD128, Iop_D128HItoD64, Iop_D128LOtoD64,
      Iop_DPBtoBCD,
      Iop_BCDtoDPB,
      Iop_BCDAdd, Iop_BCDSub,
      Iop_I128StoBCD128,
      Iop_BCD128toI128S,
      Iop_ReinterpI64asD64,
      Iop_ReinterpD64asI64,
      Iop_Add32Fx4, Iop_Sub32Fx4, Iop_Mul32Fx4, Iop_Div32Fx4,
      Iop_Max32Fx4, Iop_Min32Fx4,
      Iop_Add32Fx2, Iop_Sub32Fx2,
      Iop_CmpEQ32Fx4, Iop_CmpLT32Fx4, Iop_CmpLE32Fx4, Iop_CmpUN32Fx4,
      Iop_CmpGT32Fx4, Iop_CmpGE32Fx4,
      Iop_PwMax32Fx4, Iop_PwMin32Fx4,
      Iop_Abs32Fx4,
      Iop_Neg32Fx4,
      Iop_Sqrt32Fx4,
      Iop_RecipEst32Fx4,
      Iop_RecipStep32Fx4,
      Iop_RSqrtEst32Fx4,
      Iop_RSqrtStep32Fx4,
      Iop_I32UtoFx4, Iop_I32StoFx4,
      Iop_FtoI32Ux4_RZ, Iop_FtoI32Sx4_RZ,
      Iop_QFtoI32Ux4_RZ, Iop_QFtoI32Sx4_RZ,
      Iop_RoundF32x4_RM, Iop_RoundF32x4_RP,
      Iop_RoundF32x4_RN, Iop_RoundF32x4_RZ,
      Iop_F32ToFixed32Ux4_RZ, Iop_F32ToFixed32Sx4_RZ,
      Iop_Fixed32UToF32x4_RN, Iop_Fixed32SToF32x4_RN,
      Iop_F32toF16x4, Iop_F16toF32x4,
      Iop_F64toF16x2, Iop_F16toF64x2,
      Iop_Add32F0x4, Iop_Sub32F0x4, Iop_Mul32F0x4, Iop_Div32F0x4,
      Iop_Max32F0x4, Iop_Min32F0x4,
      Iop_CmpEQ32F0x4, Iop_CmpLT32F0x4, Iop_CmpLE32F0x4, Iop_CmpUN32F0x4,
      Iop_RecipEst32F0x4, Iop_Sqrt32F0x4, Iop_RSqrtEst32F0x4,
      Iop_Add64Fx2, Iop_Sub64Fx2, Iop_Mul64Fx2, Iop_Div64Fx2,
      Iop_Max64Fx2, Iop_Min64Fx2,
      Iop_CmpEQ64Fx2, Iop_CmpLT64Fx2, Iop_CmpLE64Fx2, Iop_CmpUN64Fx2,
      Iop_Abs64Fx2,
      Iop_Neg64Fx2,
      Iop_Sqrt64Fx2,
      Iop_RecipEst64Fx2,
      Iop_RecipStep64Fx2,
      Iop_RSqrtEst64Fx2,
      Iop_RSqrtStep64Fx2,
      Iop_Add64F0x2, Iop_Sub64F0x2, Iop_Mul64F0x2, Iop_Div64F0x2,
      Iop_Max64F0x2, Iop_Min64F0x2,
      Iop_CmpEQ64F0x2, Iop_CmpLT64F0x2, Iop_CmpLE64F0x2, Iop_CmpUN64F0x2,
      Iop_Sqrt64F0x2,
      Iop_V128to64,
      Iop_V128HIto64,
      Iop_64HLtoV128,
      Iop_64UtoV128,
      Iop_SetV128lo64,
      Iop_ZeroHI64ofV128,
      Iop_ZeroHI96ofV128,
      Iop_ZeroHI112ofV128,
      Iop_ZeroHI120ofV128,
      Iop_32UtoV128,
      Iop_V128to32,
      Iop_SetV128lo32,
      Iop_NotV128,
      Iop_AndV128, Iop_OrV128, Iop_XorV128,
      Iop_ShlV128, Iop_ShrV128, Iop_SarV128,
      Iop_CmpNEZ8x16, Iop_CmpNEZ16x8, Iop_CmpNEZ32x4, Iop_CmpNEZ64x2,
      Iop_CmpNEZ128x1,
      Iop_Add8x16, Iop_Add16x8, Iop_Add32x4, Iop_Add64x2, Iop_Add128x1,
      Iop_QAdd8Ux16, Iop_QAdd16Ux8, Iop_QAdd32Ux4, Iop_QAdd64Ux2,
      Iop_QAdd8Sx16, Iop_QAdd16Sx8, Iop_QAdd32Sx4, Iop_QAdd64Sx2,
      Iop_QAddExtUSsatSS8x16, Iop_QAddExtUSsatSS16x8,
      Iop_QAddExtUSsatSS32x4, Iop_QAddExtUSsatSS64x2,
      Iop_QAddExtSUsatUU8x16, Iop_QAddExtSUsatUU16x8,
      Iop_QAddExtSUsatUU32x4, Iop_QAddExtSUsatUU64x2,
      Iop_Sub8x16, Iop_Sub16x8, Iop_Sub32x4, Iop_Sub64x2, Iop_Sub128x1,
      Iop_QSub8Ux16, Iop_QSub16Ux8, Iop_QSub32Ux4, Iop_QSub64Ux2,
      Iop_QSub8Sx16, Iop_QSub16Sx8, Iop_QSub32Sx4, Iop_QSub64Sx2,
      Iop_Mul8x16, Iop_Mul16x8, Iop_Mul32x4,
      Iop_MulHi8Ux16, Iop_MulHi16Ux8, Iop_MulHi32Ux4,
      Iop_MulHi8Sx16, Iop_MulHi16Sx8, Iop_MulHi32Sx4,
      Iop_MullEven8Ux16, Iop_MullEven16Ux8, Iop_MullEven32Ux4,
      Iop_MullEven8Sx16, Iop_MullEven16Sx8, Iop_MullEven32Sx4,
      Iop_Mull8Ux8, Iop_Mull8Sx8,
      Iop_Mull16Ux4, Iop_Mull16Sx4,
      Iop_Mull32Ux2, Iop_Mull32Sx2,
      Iop_QDMull16Sx4, Iop_QDMull32Sx2,
      Iop_QDMulHi16Sx8, Iop_QDMulHi32Sx4,
      Iop_QRDMulHi16Sx8, Iop_QRDMulHi32Sx4,
      Iop_PolynomialMul8x16,
      Iop_PolynomialMull8x8,
      Iop_PolynomialMulAdd8x16, Iop_PolynomialMulAdd16x8,
      Iop_PolynomialMulAdd32x4, Iop_PolynomialMulAdd64x2,
      Iop_PwAdd8x16, Iop_PwAdd16x8, Iop_PwAdd32x4,
      Iop_PwAdd32Fx2,
      Iop_PwAddL8Ux16, Iop_PwAddL16Ux8, Iop_PwAddL32Ux4, Iop_PwAddL64Ux2,
      Iop_PwAddL8Sx16, Iop_PwAddL16Sx8, Iop_PwAddL32Sx4,
      Iop_PwBitMtxXpose64x2,
      Iop_Abs8x16, Iop_Abs16x8, Iop_Abs32x4, Iop_Abs64x2,
      Iop_Avg8Ux16, Iop_Avg16Ux8, Iop_Avg32Ux4, Iop_Avg64Ux2,
      Iop_Avg8Sx16, Iop_Avg16Sx8, Iop_Avg32Sx4, Iop_Avg64Sx2,
      Iop_Max8Sx16, Iop_Max16Sx8, Iop_Max32Sx4, Iop_Max64Sx2,
      Iop_Max8Ux16, Iop_Max16Ux8, Iop_Max32Ux4, Iop_Max64Ux2,
      Iop_Min8Sx16, Iop_Min16Sx8, Iop_Min32Sx4, Iop_Min64Sx2,
      Iop_Min8Ux16, Iop_Min16Ux8, Iop_Min32Ux4, Iop_Min64Ux2,
      Iop_CmpEQ8x16, Iop_CmpEQ16x8, Iop_CmpEQ32x4, Iop_CmpEQ64x2,
      Iop_CmpGT8Sx16, Iop_CmpGT16Sx8, Iop_CmpGT32Sx4, Iop_CmpGT64Sx2,
      Iop_CmpGT8Ux16, Iop_CmpGT16Ux8, Iop_CmpGT32Ux4, Iop_CmpGT64Ux2,
      Iop_Cnt8x16,
      Iop_Clz8x16, Iop_Clz16x8, Iop_Clz32x4,
      Iop_Cls8x16, Iop_Cls16x8, Iop_Cls32x4,
      Iop_ShlN8x16, Iop_ShlN16x8, Iop_ShlN32x4, Iop_ShlN64x2,
      Iop_ShrN8x16, Iop_ShrN16x8, Iop_ShrN32x4, Iop_ShrN64x2,
      Iop_SarN8x16, Iop_SarN16x8, Iop_SarN32x4, Iop_SarN64x2,
      Iop_Shl8x16, Iop_Shl16x8, Iop_Shl32x4, Iop_Shl64x2,
      Iop_Shr8x16, Iop_Shr16x8, Iop_Shr32x4, Iop_Shr64x2,
      Iop_Sar8x16, Iop_Sar16x8, Iop_Sar32x4, Iop_Sar64x2,
      Iop_Sal8x16, Iop_Sal16x8, Iop_Sal32x4, Iop_Sal64x2,
      Iop_Rol8x16, Iop_Rol16x8, Iop_Rol32x4, Iop_Rol64x2,
      Iop_QShl8x16, Iop_QShl16x8, Iop_QShl32x4, Iop_QShl64x2,
      Iop_QSal8x16, Iop_QSal16x8, Iop_QSal32x4, Iop_QSal64x2,
      Iop_QShlNsatSU8x16, Iop_QShlNsatSU16x8,
      Iop_QShlNsatSU32x4, Iop_QShlNsatSU64x2,
      Iop_QShlNsatUU8x16, Iop_QShlNsatUU16x8,
      Iop_QShlNsatUU32x4, Iop_QShlNsatUU64x2,
      Iop_QShlNsatSS8x16, Iop_QShlNsatSS16x8,
      Iop_QShlNsatSS32x4, Iop_QShlNsatSS64x2,
      Iop_QandUQsh8x16, Iop_QandUQsh16x8,
      Iop_QandUQsh32x4, Iop_QandUQsh64x2,
      Iop_QandSQsh8x16, Iop_QandSQsh16x8,
      Iop_QandSQsh32x4, Iop_QandSQsh64x2,
      Iop_QandUQRsh8x16, Iop_QandUQRsh16x8,
      Iop_QandUQRsh32x4, Iop_QandUQRsh64x2,
      Iop_QandSQRsh8x16, Iop_QandSQRsh16x8,
      Iop_QandSQRsh32x4, Iop_QandSQRsh64x2,
      Iop_Sh8Sx16, Iop_Sh16Sx8, Iop_Sh32Sx4, Iop_Sh64Sx2,
      Iop_Sh8Ux16, Iop_Sh16Ux8, Iop_Sh32Ux4, Iop_Sh64Ux2,
      Iop_Rsh8Sx16, Iop_Rsh16Sx8, Iop_Rsh32Sx4, Iop_Rsh64Sx2,
      Iop_Rsh8Ux16, Iop_Rsh16Ux8, Iop_Rsh32Ux4, Iop_Rsh64Ux2,
      Iop_QandQShrNnarrow16Uto8Ux8,
      Iop_QandQShrNnarrow32Uto16Ux4, Iop_QandQShrNnarrow64Uto32Ux2,
      Iop_QandQSarNnarrow16Sto8Sx8,
      Iop_QandQSarNnarrow32Sto16Sx4, Iop_QandQSarNnarrow64Sto32Sx2,
      Iop_QandQSarNnarrow16Sto8Ux8,
      Iop_QandQSarNnarrow32Sto16Ux4, Iop_QandQSarNnarrow64Sto32Ux2,
      Iop_QandQRShrNnarrow16Uto8Ux8,
      Iop_QandQRShrNnarrow32Uto16Ux4, Iop_QandQRShrNnarrow64Uto32Ux2,
      Iop_QandQRSarNnarrow16Sto8Sx8,
      Iop_QandQRSarNnarrow32Sto16Sx4, Iop_QandQRSarNnarrow64Sto32Sx2,
      Iop_QandQRSarNnarrow16Sto8Ux8,
      Iop_QandQRSarNnarrow32Sto16Ux4, Iop_QandQRSarNnarrow64Sto32Ux2,
      Iop_QNarrowBin16Sto8Ux16, Iop_QNarrowBin32Sto16Ux8,
      Iop_QNarrowBin16Sto8Sx16, Iop_QNarrowBin32Sto16Sx8,
      Iop_QNarrowBin16Uto8Ux16, Iop_QNarrowBin32Uto16Ux8,
      Iop_NarrowBin16to8x16, Iop_NarrowBin32to16x8,
      Iop_QNarrowBin64Sto32Sx4, Iop_QNarrowBin64Uto32Ux4,
      Iop_NarrowBin64to32x4,
      Iop_NarrowUn16to8x8, Iop_NarrowUn32to16x4, Iop_NarrowUn64to32x2,
      Iop_QNarrowUn16Sto8Sx8, Iop_QNarrowUn32Sto16Sx4, Iop_QNarrowUn64Sto32Sx2,
      Iop_QNarrowUn16Sto8Ux8, Iop_QNarrowUn32Sto16Ux4, Iop_QNarrowUn64Sto32Ux2,
      Iop_QNarrowUn16Uto8Ux8, Iop_QNarrowUn32Uto16Ux4, Iop_QNarrowUn64Uto32Ux2,
      Iop_Widen8Uto16x8, Iop_Widen16Uto32x4, Iop_Widen32Uto64x2,
      Iop_Widen8Sto16x8, Iop_Widen16Sto32x4, Iop_Widen32Sto64x2,
      Iop_InterleaveHI8x16, Iop_InterleaveHI16x8,
      Iop_InterleaveHI32x4, Iop_InterleaveHI64x2,
      Iop_InterleaveLO8x16, Iop_InterleaveLO16x8,
      Iop_InterleaveLO32x4, Iop_InterleaveLO64x2,
      Iop_InterleaveOddLanes8x16, Iop_InterleaveEvenLanes8x16,
      Iop_InterleaveOddLanes16x8, Iop_InterleaveEvenLanes16x8,
      Iop_InterleaveOddLanes32x4, Iop_InterleaveEvenLanes32x4,
      Iop_CatOddLanes8x16, Iop_CatOddLanes16x8, Iop_CatOddLanes32x4,
      Iop_CatEvenLanes8x16, Iop_CatEvenLanes16x8, Iop_CatEvenLanes32x4,
      Iop_GetElem8x16, Iop_GetElem16x8, Iop_GetElem32x4, Iop_GetElem64x2,
      Iop_SetElem8x16, Iop_SetElem16x8, Iop_SetElem32x4, Iop_SetElem64x2,
      Iop_Dup8x16, Iop_Dup16x8, Iop_Dup32x4,
      Iop_SliceV128,
      Iop_Reverse8sIn16_x8,
      Iop_Reverse8sIn32_x4, Iop_Reverse16sIn32_x4,
      Iop_Reverse8sIn64_x2, Iop_Reverse16sIn64_x2, Iop_Reverse32sIn64_x2,
      Iop_Reverse1sIn8_x16,
      Iop_Perm8x16,
      Iop_Perm32x4,
      Iop_Perm8x16x2,
      Iop_GetMSBs8x16,
      Iop_RecipEst32Ux4, Iop_RSqrtEst32Ux4,
      Iop_MulI128by10,
      Iop_MulI128by10Carry,
      Iop_MulI128by10E,
      Iop_MulI128by10ECarry,
      Iop_V256to64_0,
      Iop_V256to64_1,
      Iop_V256to64_2,
      Iop_V256to64_3,
      Iop_64x4toV256,
      Iop_V256toV128_0,
      Iop_V256toV128_1,
      Iop_V128HLtoV256,
      Iop_AndV256,
      Iop_OrV256,
      Iop_XorV256,
      Iop_NotV256,
      Iop_CmpNEZ8x32, Iop_CmpNEZ16x16, Iop_CmpNEZ32x8, Iop_CmpNEZ64x4,
      Iop_Add8x32, Iop_Add16x16, Iop_Add32x8, Iop_Add64x4,
      Iop_Sub8x32, Iop_Sub16x16, Iop_Sub32x8, Iop_Sub64x4,
      Iop_CmpEQ8x32, Iop_CmpEQ16x16, Iop_CmpEQ32x8, Iop_CmpEQ64x4,
      Iop_CmpGT8Sx32, Iop_CmpGT16Sx16, Iop_CmpGT32Sx8, Iop_CmpGT64Sx4,
      Iop_ShlN16x16, Iop_ShlN32x8, Iop_ShlN64x4,
      Iop_ShrN16x16, Iop_ShrN32x8, Iop_ShrN64x4,
      Iop_SarN16x16, Iop_SarN32x8,
      Iop_Max8Sx32, Iop_Max16Sx16, Iop_Max32Sx8,
      Iop_Max8Ux32, Iop_Max16Ux16, Iop_Max32Ux8,
      Iop_Min8Sx32, Iop_Min16Sx16, Iop_Min32Sx8,
      Iop_Min8Ux32, Iop_Min16Ux16, Iop_Min32Ux8,
      Iop_Mul16x16, Iop_Mul32x8,
      Iop_MulHi16Ux16, Iop_MulHi16Sx16,
      Iop_QAdd8Ux32, Iop_QAdd16Ux16,
      Iop_QAdd8Sx32, Iop_QAdd16Sx16,
      Iop_QSub8Ux32, Iop_QSub16Ux16,
      Iop_QSub8Sx32, Iop_QSub16Sx16,
      Iop_Avg8Ux32, Iop_Avg16Ux16,
      Iop_Perm32x8,
      Iop_CipherV128, Iop_CipherLV128, Iop_CipherSV128,
      Iop_NCipherV128, Iop_NCipherLV128,
      Iop_SHA512, Iop_SHA256,
      Iop_Add64Fx4, Iop_Sub64Fx4, Iop_Mul64Fx4, Iop_Div64Fx4,
      Iop_Add32Fx8, Iop_Sub32Fx8, Iop_Mul32Fx8, Iop_Div32Fx8,
      Iop_Sqrt32Fx8,
      Iop_Sqrt64Fx4,
      Iop_RSqrtEst32Fx8,
      Iop_RecipEst32Fx8,
      Iop_Max32Fx8, Iop_Min32Fx8,
      Iop_Max64Fx4, Iop_Min64Fx4,
      Iop_LAST
   }
   IROp;
extern void ppIROp ( IROp );
extern void typeOfPrimop ( IROp op,
                                    IRType* t_dst, IRType* t_arg1,
                           IRType* t_arg2, IRType* t_arg3, IRType* t_arg4 );
typedef
   enum {
      Irrm_NEAREST = 0,
      Irrm_NegINF = 1,
      Irrm_PosINF = 2,
      Irrm_ZERO = 3,
      Irrm_NEAREST_TIE_AWAY_0 = 4,
      Irrm_PREPARE_SHORTER = 5,
      Irrm_AWAY_FROM_ZERO = 6,
      Irrm_NEAREST_TIE_TOWARD_0 = 7,
      Irrm_INVALID = 8
   }
   IRRoundingMode;
typedef
   enum {
      Ircr_UN = 0x45,
      Ircr_LT = 0x01,
      Ircr_GT = 0x00,
      Ircr_EQ = 0x40
   }
   IRCmpFResult;
typedef IRCmpFResult IRCmpF32Result;
typedef IRCmpFResult IRCmpF64Result;
typedef IRCmpFResult IRCmpF128Result;
typedef IRCmpFResult IRCmpDResult;
typedef IRCmpDResult IRCmpD64Result;
typedef IRCmpDResult IRCmpD128Result;
typedef struct _IRQop IRQop;
typedef struct _IRTriop IRTriop;
typedef
   enum {
      Iex_Binder=0x1900,
      Iex_Get,
      Iex_GetI,
      Iex_RdTmp,
      Iex_Qop,
      Iex_Triop,
      Iex_Binop,
      Iex_Unop,
      Iex_Load,
      Iex_Const,
      Iex_ITE,
      Iex_CCall,
      Iex_VECRET,
      Iex_GSPTR
   }
   IRExprTag;
typedef
   struct _IRExpr
   IRExpr;
struct _IRExpr {
   IRExprTag tag;
   union {
      struct {
         Int binder;
      } Binder;
      struct {
         Int offset;
         IRType ty;
      } Get;
      struct {
         IRRegArray* descr;
         IRExpr* ix;
         Int bias;
      } GetI;
      struct {
         IRTemp tmp;
      } RdTmp;
      struct {
        IRQop* details;
      } Qop;
      struct {
        IRTriop* details;
      } Triop;
      struct {
         IROp op;
         IRExpr* arg1;
         IRExpr* arg2;
      } Binop;
      struct {
         IROp op;
         IRExpr* arg;
      } Unop;
      struct {
         IREndness end;
         IRType ty;
         IRExpr* addr;
      } Load;
      struct {
         IRConst* con;
      } Const;
      struct {
         IRCallee* cee;
         IRType retty;
         IRExpr** args;
      } CCall;
      struct {
         IRExpr* cond;
         IRExpr* iftrue;
         IRExpr* iffalse;
      } ITE;
   } Iex;
};
struct _IRTriop {
   IROp op;
   IRExpr* arg1;
   IRExpr* arg2;
   IRExpr* arg3;
};
struct _IRQop {
   IROp op;
   IRExpr* arg1;
   IRExpr* arg2;
   IRExpr* arg3;
   IRExpr* arg4;
};
extern IRExpr* IRExpr_Binder ( Int binder );
extern IRExpr* IRExpr_Get ( Int off, IRType ty );
extern IRExpr* IRExpr_GetI ( IRRegArray* descr, IRExpr* ix, Int bias );
extern IRExpr* IRExpr_RdTmp ( IRTemp tmp );
extern IRExpr* IRExpr_Qop ( IROp op, IRExpr* arg1, IRExpr* arg2,
                                        IRExpr* arg3, IRExpr* arg4 );
extern IRExpr* IRExpr_Triop ( IROp op, IRExpr* arg1,
                                        IRExpr* arg2, IRExpr* arg3 );
extern IRExpr* IRExpr_Binop ( IROp op, IRExpr* arg1, IRExpr* arg2 );
extern IRExpr* IRExpr_Unop ( IROp op, IRExpr* arg );
extern IRExpr* IRExpr_Load ( IREndness end, IRType ty, IRExpr* addr );
extern IRExpr* IRExpr_Const ( IRConst* con );
extern IRExpr* IRExpr_CCall ( IRCallee* cee, IRType retty, IRExpr** args );
extern IRExpr* IRExpr_ITE ( IRExpr* cond, IRExpr* iftrue, IRExpr* iffalse );
extern IRExpr* IRExpr_VECRET ( void );
extern IRExpr* IRExpr_GSPTR ( void );
extern IRExpr* deepCopyIRExpr ( const IRExpr* );
extern void ppIRExpr ( const IRExpr* );
extern IRExpr** mkIRExprVec_0 ( void );
extern IRExpr** mkIRExprVec_1 ( IRExpr* );
extern IRExpr** mkIRExprVec_2 ( IRExpr*, IRExpr* );
extern IRExpr** mkIRExprVec_3 ( IRExpr*, IRExpr*, IRExpr* );
extern IRExpr** mkIRExprVec_4 ( IRExpr*, IRExpr*, IRExpr*, IRExpr* );
extern IRExpr** mkIRExprVec_5 ( IRExpr*, IRExpr*, IRExpr*, IRExpr*,
                                IRExpr* );
extern IRExpr** mkIRExprVec_6 ( IRExpr*, IRExpr*, IRExpr*, IRExpr*,
                                IRExpr*, IRExpr* );
extern IRExpr** mkIRExprVec_7 ( IRExpr*, IRExpr*, IRExpr*, IRExpr*,
                                IRExpr*, IRExpr*, IRExpr* );
extern IRExpr** mkIRExprVec_8 ( IRExpr*, IRExpr*, IRExpr*, IRExpr*,
                                IRExpr*, IRExpr*, IRExpr*, IRExpr* );
extern IRExpr** mkIRExprVec_9 ( IRExpr*, IRExpr*, IRExpr*, IRExpr*,
                                IRExpr*, IRExpr*, IRExpr*, IRExpr*, IRExpr* );
extern IRExpr** mkIRExprVec_13 ( IRExpr*, IRExpr*, IRExpr*, IRExpr*,
                                 IRExpr*, IRExpr*, IRExpr*, IRExpr*,
                                 IRExpr*, IRExpr*, IRExpr*, IRExpr*, IRExpr* );
extern IRExpr** shallowCopyIRExprVec ( IRExpr** );
extern IRExpr** deepCopyIRExprVec ( IRExpr *const * );
extern IRExpr* mkIRExpr_HWord ( HWord );
extern
IRExpr* mkIRExprCCall ( IRType retty,
                        Int regparms, const HChar* name, void* addr,
                        IRExpr** args );
extern Bool eqIRAtom ( const IRExpr*, const IRExpr* );
typedef
   enum {
      Ijk_INVALID=0x1A00,
      Ijk_Boring,
      Ijk_Call,
      Ijk_Ret,
      Ijk_ClientReq,
      Ijk_Yield,
      Ijk_EmWarn,
      Ijk_EmFail,
      Ijk_NoDecode,
      Ijk_MapFail,
      Ijk_InvalICache,
      Ijk_FlushDCache,
      Ijk_NoRedir,
      Ijk_SigILL,
      Ijk_SigTRAP,
      Ijk_SigSEGV,
      Ijk_SigBUS,
      Ijk_SigFPE,
      Ijk_SigFPE_IntDiv,
      Ijk_SigFPE_IntOvf,
      Ijk_Privileged,
      Ijk_Sys_syscall,
      Ijk_Sys_int,
      Ijk_Sys_int32,
      Ijk_Sys_int128,
      Ijk_Sys_int129,
      Ijk_Sys_int130,
      Ijk_Sys_int145,
      Ijk_Sys_int210,
      Ijk_Sys_sysenter
   }
   IRJumpKind;
extern void ppIRJumpKind ( IRJumpKind );
typedef
   enum {
      Ifx_None=0x1B00,
      Ifx_Read,
      Ifx_Write,
      Ifx_Modify,
   }
   IREffect;
extern void ppIREffect ( IREffect );
typedef
   struct _IRDirty {
      IRCallee* cee;
      IRExpr* guard;
      IRExpr** args;
      IRTemp tmp;
      IREffect mFx;
      IRExpr* mAddr;
      Int mSize;
      Int nFxState;
      struct {
         IREffect fx:16;
         UShort offset;
         UShort size;
         UChar nRepeats;
         UChar repeatLen;
      } fxState[7];
   }
   IRDirty;
extern void ppIRDirty ( const IRDirty* );
extern IRDirty* emptyIRDirty ( void );
extern IRDirty* deepCopyIRDirty ( const IRDirty* );
extern
IRDirty* unsafeIRDirty_0_N ( Int regparms, const HChar* name, void* addr,
                             IRExpr** args );
extern
IRDirty* unsafeIRDirty_1_N ( IRTemp dst,
                             Int regparms, const HChar* name, void* addr,
                             IRExpr** args );
typedef
   enum {
      Imbe_Fence=0x1C00,
      Imbe_CancelReservation
   }
   IRMBusEvent;
extern void ppIRMBusEvent ( IRMBusEvent );
typedef
   struct {
      IRTemp oldHi;
      IRTemp oldLo;
      IREndness end;
      IRExpr* addr;
      IRExpr* expdHi;
      IRExpr* expdLo;
      IRExpr* dataHi;
      IRExpr* dataLo;
   }
   IRCAS;
extern void ppIRCAS ( const IRCAS* cas );
extern IRCAS* mkIRCAS ( IRTemp oldHi, IRTemp oldLo,
                        IREndness end, IRExpr* addr,
                        IRExpr* expdHi, IRExpr* expdLo,
                        IRExpr* dataHi, IRExpr* dataLo );
extern IRCAS* deepCopyIRCAS ( const IRCAS* );
typedef
   struct {
      IRRegArray* descr;
      IRExpr* ix;
      Int bias;
      IRExpr* data;
   } IRPutI;
extern void ppIRPutI ( const IRPutI* puti );
extern IRPutI* mkIRPutI ( IRRegArray* descr, IRExpr* ix,
                          Int bias, IRExpr* data );
extern IRPutI* deepCopyIRPutI ( const IRPutI* );
typedef
   struct {
      IREndness end;
      IRExpr* addr;
      IRExpr* data;
      IRExpr* guard;
   }
   IRStoreG;
typedef
   enum {
      ILGop_INVALID=0x1D00,
      ILGop_IdentV128,
      ILGop_Ident64,
      ILGop_Ident32,
      ILGop_16Uto32,
      ILGop_16Sto32,
      ILGop_8Uto32,
      ILGop_8Sto32
   }
   IRLoadGOp;
typedef
   struct {
      IREndness end;
      IRLoadGOp cvt;
      IRTemp dst;
      IRExpr* addr;
      IRExpr* alt;
      IRExpr* guard;
   }
   IRLoadG;
extern void ppIRStoreG ( const IRStoreG* sg );
extern void ppIRLoadGOp ( IRLoadGOp cvt );
extern void ppIRLoadG ( const IRLoadG* lg );
extern IRStoreG* mkIRStoreG ( IREndness end,
                              IRExpr* addr, IRExpr* data,
                              IRExpr* guard );
extern IRLoadG* mkIRLoadG ( IREndness end, IRLoadGOp cvt,
                            IRTemp dst, IRExpr* addr, IRExpr* alt,
                            IRExpr* guard );
typedef
   enum {
      Ist_NoOp=0x1E00,
      Ist_IMark,
      Ist_AbiHint,
      Ist_Put,
      Ist_PutI,
      Ist_WrTmp,
      Ist_Store,
      Ist_LoadG,
      Ist_StoreG,
      Ist_CAS,
      Ist_LLSC,
      Ist_Dirty,
      Ist_MBE,
      Ist_Exit
   }
   IRStmtTag;
typedef
   struct _IRStmt {
      IRStmtTag tag;
      union {
         struct {
    UInt dummy;
  } NoOp;
         struct {
            Addr addr;
            UInt len;
            UChar delta;
         } IMark;
         struct {
            IRExpr* base;
            Int len;
            IRExpr* nia;
         } AbiHint;
         struct {
            Int offset;
            IRExpr* data;
         } Put;
         struct {
            IRPutI* details;
         } PutI;
         struct {
            IRTemp tmp;
            IRExpr* data;
         } WrTmp;
         struct {
            IREndness end;
            IRExpr* addr;
            IRExpr* data;
         } Store;
         struct {
            IRStoreG* details;
         } StoreG;
         struct {
            IRLoadG* details;
         } LoadG;
         struct {
            IRCAS* details;
         } CAS;
         struct {
            IREndness end;
            IRTemp result;
            IRExpr* addr;
            IRExpr* storedata;
         } LLSC;
         struct {
            IRDirty* details;
         } Dirty;
         struct {
            IRMBusEvent event;
         } MBE;
         struct {
            IRExpr* guard;
            IRConst* dst;
            IRJumpKind jk;
            Int offsIP;
         } Exit;
      } Ist;
   }
   IRStmt;
extern IRStmt* IRStmt_NoOp ( void );
extern IRStmt* IRStmt_IMark ( Addr addr, UInt len, UChar delta );
extern IRStmt* IRStmt_AbiHint ( IRExpr* base, Int len, IRExpr* nia );
extern IRStmt* IRStmt_Put ( Int off, IRExpr* data );
extern IRStmt* IRStmt_PutI ( IRPutI* details );
extern IRStmt* IRStmt_WrTmp ( IRTemp tmp, IRExpr* data );
extern IRStmt* IRStmt_Store ( IREndness end, IRExpr* addr, IRExpr* data );
extern IRStmt* IRStmt_StoreG ( IREndness end, IRExpr* addr, IRExpr* data,
                                IRExpr* guard );
extern IRStmt* IRStmt_LoadG ( IREndness end, IRLoadGOp cvt, IRTemp dst,
                                IRExpr* addr, IRExpr* alt, IRExpr* guard );
extern IRStmt* IRStmt_CAS ( IRCAS* details );
extern IRStmt* IRStmt_LLSC ( IREndness end, IRTemp result,
                                IRExpr* addr, IRExpr* storedata );
extern IRStmt* IRStmt_Dirty ( IRDirty* details );
extern IRStmt* IRStmt_MBE ( IRMBusEvent event );
extern IRStmt* IRStmt_Exit ( IRExpr* guard, IRJumpKind jk, IRConst* dst,
                                Int offsIP );
extern IRStmt* deepCopyIRStmt ( const IRStmt* );
extern void ppIRStmt ( const IRStmt* );
typedef
   struct {
      IRType* types;
      Int types_size;
      Int types_used;
   }
   IRTypeEnv;
extern IRTemp newIRTemp ( IRTypeEnv*, IRType );
extern IRTypeEnv* deepCopyIRTypeEnv ( const IRTypeEnv* );
extern void ppIRTypeEnv ( const IRTypeEnv* );
typedef
   struct {
      IRTypeEnv* tyenv;
      IRStmt** stmts;
      Int stmts_size;
      Int stmts_used;
      IRExpr* next;
      IRJumpKind jumpkind;
      Int offsIP;
   }
   IRSB;
extern IRSB* emptyIRSB ( void );
extern IRSB* deepCopyIRSB ( const IRSB* );
extern IRSB* deepCopyIRSBExceptStmts ( const IRSB* );
extern void ppIRSB ( const IRSB* );
extern void addStmtToIRSB ( IRSB*, IRStmt* );
extern IRTypeEnv* emptyIRTypeEnv ( void );
extern IRType typeOfIRConst ( const IRConst* );
extern IRType typeOfIRTemp ( const IRTypeEnv*, IRTemp );
extern IRType typeOfIRExpr ( const IRTypeEnv*, const IRExpr* );
extern void typeOfIRLoadGOp ( IRLoadGOp cvt,
                                     IRType* t_res,
                                     IRType* t_arg );
extern void sanityCheckIRSB ( const IRSB* bb,
                              const HChar* caller,
                              Bool require_flatness,
                              IRType guest_word_size );
extern Bool isFlatIRStmt ( const IRStmt* );
extern Bool isPlausibleIRType ( IRType ty );
void vex_inject_ir(IRSB *, IREndness);
typedef
   enum {
      VexArch_INVALID=0x400,
      VexArchX86,
      VexArchAMD64,
      VexArchARM,
      VexArchARM64,
      VexArchPPC32,
      VexArchPPC64,
      VexArchS390X,
      VexArchMIPS32,
      VexArchMIPS64,
      VexArchTILEGX,
      VexArchRISCV64
   }
   VexArch;
typedef
   enum {
      VexEndness_INVALID=0x600,
      VexEndnessLE,
      VexEndnessBE
   }
   VexEndness;
extern const HChar* LibVEX_ppVexArch ( VexArch );
extern const HChar* LibVEX_ppVexEndness ( VexEndness endness );
extern const HChar* LibVEX_ppVexHwCaps ( VexArch, UInt );
typedef enum {
   DATA_CACHE=0x500,
   INSN_CACHE,
   UNIFIED_CACHE
} VexCacheKind;
typedef struct {
   VexCacheKind kind;
   UInt level;
   UInt sizeB;
   UInt line_sizeB;
   UInt assoc;
   Bool is_trace_cache;
} VexCache;
typedef struct {
   UInt num_levels;
   UInt num_caches;
   VexCache *caches;
   Bool icaches_maintain_coherence;
} VexCacheInfo;
typedef
   struct {
      UInt hwcaps;
      VexEndness endness;
      VexCacheInfo hwcache_info;
      Int ppc_icache_line_szB;
      UInt ppc_dcbz_szB;
      UInt ppc_dcbzl_szB;
      UInt arm64_dMinLine_lg2_szB;
      UInt arm64_iMinLine_lg2_szB;
      UInt x86_cr0;
   }
   VexArchInfo;
extern
void LibVEX_default_VexArchInfo ( VexArchInfo* vai );
typedef
   struct {
      Int guest_stack_redzone_size;
      Bool guest_amd64_assume_fs_is_const;
      Bool guest_amd64_assume_gs_is_const;
      Bool guest_ppc_zap_RZ_at_blr;
      Bool (*guest_ppc_zap_RZ_at_bl)(Addr);
      Bool host_ppc_calls_use_fndescrs;
      Bool guest_mips_fp_mode64;
      Bool guest__use_fallback_LLSC;
   }
   VexAbiInfo;
extern
void LibVEX_default_VexAbiInfo ( VexAbiInfo* vbi );
typedef
   enum {
      VexRegUpd_INVALID=0x700,
      VexRegUpdSpAtMemAccess,
      VexRegUpdUnwindregsAtMemAccess,
      VexRegUpdAllregsAtMemAccess,
      VexRegUpdAllregsAtEachInsn,
      VexRegUpdLdAllregsAtEachInsn
   }
   VexRegisterUpdates;
typedef
   struct {
      Int iropt_verbosity;
      Int iropt_level;
      VexRegisterUpdates iropt_register_updates_default;
      Int iropt_unroll_thresh;
      Int guest_max_insns;
      Int guest_max_bytes;
      Int guest_chase_thresh;
      Bool guest_chase_cond;
      UInt regalloc_version;
      Bool arm_allow_optimizing_lookback;
      Bool strict_block_end;
      Bool arm64_allow_reordered_writeback;
      Bool x86_optimize_callpop_idiom;
      Bool special_instruction_support;
      UInt lookback_amount;
   }
   VexControl;
extern
void LibVEX_default_VexControl ( VexControl* vcon );
extern void* LibVEX_Alloc ( SizeT nbytes );
extern void LibVEX_ShowAllocStats ( void );
typedef
   struct {
      Int total_sizeB;
      Int offset_SP;
      Int sizeof_SP;
      Int offset_FP;
      Int sizeof_FP;
      Int offset_IP;
      Int sizeof_IP;
      Int n_alwaysDefd;
      struct {
         Int offset;
         Int size;
      } alwaysDefd[24];
   }
   VexGuestLayout;
extern void LibVEX_Init (
   
   void (*failure_exit) ( void ),
   void (*log_bytes) ( const HChar*, SizeT nbytes ),
   Int debuglevel,
   const VexControl* vcon
);
extern void LibVEX_Update_Control (const VexControl * );
typedef
   struct {
      enum { VexTransOK=0x800,
             VexTransAccessFail, VexTransOutputFull } status;
      UInt n_sc_extents;
      Int offs_profInc;
      UInt n_guest_instrs;
   }
   VexTranslateResult;
typedef
   struct {
      Addr base[3];
      UShort len[3];
      UShort n_used;
   }
   VexGuestExtents;
typedef
   struct {
      VexArch arch_guest;
      VexArchInfo archinfo_guest;
      VexArch arch_host;
      VexArchInfo archinfo_host;
      VexAbiInfo abiinfo_both;
      void* callback_opaque;
      const UChar* guest_bytes;
      Addr guest_bytes_addr;
      Bool (*chase_into_ok) ( void*, Addr );
      VexGuestExtents* guest_extents;
      UChar* host_bytes;
      Int host_bytes_size;
      Int* host_bytes_used;
      IRSB* (*instrument1) ( void*,
                               IRSB*,
                               const VexGuestLayout*,
                               const VexGuestExtents*,
                               const VexArchInfo*,
                               IRType gWordTy, IRType hWordTy );
      IRSB* (*instrument2) ( void*,
                               IRSB*,
                               const VexGuestLayout*,
                               const VexGuestExtents*,
                               const VexArchInfo*,
                               IRType gWordTy, IRType hWordTy );
      IRSB* (*finaltidy) ( IRSB* );
      UInt (*needs_self_check)( void*,
                                             VexRegisterUpdates* pxControl,
                                const VexGuestExtents* );
      Bool (*preamble_function)( void*, IRSB*);
      Int traceflags;
      Bool sigill_diag;
      Bool addProfInc;
      const void* disp_cp_chain_me_to_slowEP;
      const void* disp_cp_chain_me_to_fastEP;
      const void* disp_cp_xindir;
      const void* disp_cp_xassisted;
   }
   VexTranslateArgs;
extern
VexTranslateResult LibVEX_Translate ( VexTranslateArgs* );
extern
IRSB *LibVEX_Lift ( VexTranslateArgs*,
                    VexTranslateResult*,
                    VexRegisterUpdates* );
extern
void LibVEX_Codegen ( VexTranslateArgs*,
                      VexTranslateResult*,
                      IRSB*,
                      VexRegisterUpdates );
typedef
   struct {
      HWord start;
      HWord len;
   }
   VexInvalRange;
extern
VexInvalRange LibVEX_Chain ( VexArch arch_host,
                             VexEndness endhess_host,
                             void* place_to_chain,
                             const void* disp_cp_chain_me_EXPECTED,
                             const void* place_to_jump_to );
extern
VexInvalRange LibVEX_UnChain ( VexArch arch_host,
                               VexEndness endness_host,
                               void* place_to_unchain,
                               const void* place_to_jump_to_EXPECTED,
                               const void* disp_cp_chain_me );
extern
Int LibVEX_evCheckSzB ( VexArch arch_host );
extern
VexInvalRange LibVEX_PatchProfInc ( VexArch arch_host,
                                    VexEndness endness_host,
                                    void* place_to_patch,
                                    const ULong* location_of_counter );
extern void LibVEX_ShowStats ( void );
typedef
   struct {
      IROp op;
      HWord result;
      HWord opnd1;
      HWord opnd2;
      HWord opnd3;
      HWord opnd4;
      IRType t_result;
      IRType t_opnd1;
      IRType t_opnd2;
      IRType t_opnd3;
      IRType t_opnd4;
      UInt rounding_mode;
      UInt num_operands;
      UInt immediate_type;
      UInt immediate_index;
   }
   IRICB;
extern void LibVEX_InitIRI ( const IRICB * );
extern int log_level;
extern VexTranslateArgs vta;
extern char *msg_buffer;
extern size_t msg_current_size;
void clear_log(void);
int vex_init(void);
typedef struct _ExitInfo {
 Int stmt_idx;
 Addr ins_addr;
 IRStmt *stmt;
} ExitInfo;
typedef enum {
 Dt_Unknown = 0x9000,
 Dt_Integer,
 Dt_FP,
 Dt_StoreInteger
} DataRefTypes;
typedef struct _DataRef {
 Addr data_addr;
 Int size;
 DataRefTypes data_type;
 Int stmt_idx;
 Addr ins_addr;
} DataRef;
typedef struct _ConstVal {
 Int tmp;
 Int stmt_idx;
 ULong value;
} ConstVal;
typedef struct _VEXLiftResult {
 IRSB* irsb;
 Int size;
 Bool is_noop_block;
 Int exit_count;
 ExitInfo exits[400];
 Int is_default_exit_constant;
 Addr default_exit;
 Int insts;
 Addr inst_addrs[200];
 Int data_ref_count;
 DataRef data_refs[2000];
 Int const_val_count;
 ConstVal const_vals[1000];
} VEXLiftResult;
VEXLiftResult *vex_lift(
  VexArch guest,
  VexArchInfo archinfo,
  unsigned char *insn_start,
  unsigned long long insn_addr,
  unsigned int max_insns,
  unsigned int max_bytes,
  int opt_level,
  int traceflags,
  int allow_arch_optimizations,
  int strict_block_end,
  int collect_data_refs,
  int load_from_ro_regions,
  int const_prop,
  VexRegisterUpdates px_control,
  unsigned int lookback_amount);
Bool register_readonly_region(ULong start, ULong size, unsigned char* content);
void deregister_all_readonly_regions();
Bool register_initial_register_value(UInt offset, UInt size, ULong value);
Bool reset_initial_register_values();
extern VexControl vex_control;"""
guest_offsets = {('x86', 'eax'): 8, ('x86', 'ecx'): 12, ('x86', 'edx'): 16, ('x86', 'ebx'): 20, ('x86', 'esp'): 24, ('x86', 'ebp'): 28, ('x86', 'esi'): 32, ('x86', 'edi'): 36, ('x86', 'cc_op'): 40, ('x86', 'cc_dep1'): 44, ('x86', 'cc_dep2'): 48, ('x86', 'cc_ndep'): 52, ('x86', 'dflag'): 56, ('x86', 'idflag'): 60, ('x86', 'acflag'): 64, ('x86', 'eip'): 68, ('x86', 'fpreg'): 72, ('x86', 'fptag'): 136, ('x86', 'fpround'): 144, ('x86', 'fc3210'): 148, ('x86', 'ftop'): 152, ('x86', 'sseround'): 156, ('x86', 'xmm0'): 160, ('x86', 'xmm1'): 176, ('x86', 'xmm2'): 192, ('x86', 'xmm3'): 208, ('x86', 'xmm4'): 224, ('x86', 'xmm5'): 240, ('x86', 'xmm6'): 256, ('x86', 'xmm7'): 272, ('x86', 'cs'): 288, ('x86', 'ds'): 290, ('x86', 'es'): 292, ('x86', 'fs'): 294, ('x86', 'gs'): 296, ('x86', 'ss'): 298, ('x86', 'ldt'): 304, ('x86', 'gdt'): 312, ('x86', 'emnote'): 320, ('x86', 'cmstart'): 324, ('x86', 'cmlen'): 328, ('x86', 'nraddr'): 332, ('x86', 'sc_class'): 336, ('x86', 'ip_at_syscall'): 340, ('amd64', 'rax'): 16, ('amd64', 'rbx'): 40, ('amd64', 'rcx'): 24, ('amd64', 'rdx'): 32, ('amd64', 'rsi'): 64, ('amd64', 'rdi'): 72, ('amd64', 'rsp'): 48, ('amd64', 'rbp'): 56, ('amd64', 'r8'): 80, ('amd64', 'r9'): 88, ('amd64', 'r10'): 96, ('amd64', 'r11'): 104, ('amd64', 'r12'): 112, ('amd64', 'r13'): 120, ('amd64', 'r14'): 128, ('amd64', 'r15'): 136, ('amd64', 'rip'): 184, ('amd64', 'ip_at_syscall'): 1040, ('amd64', 'cc_op'): 144, ('amd64', 'cc_dep1'): 152, ('amd64', 'cc_dep2'): 160, ('amd64', 'cc_ndep'): 168, ('amd64', 'dflag'): 176, ('amd64', 'acflag'): 192, ('amd64', 'idflag'): 200, ('amd64', 'fs_const'): 208, ('amd64', 'sseround'): 216, ('amd64', 'ymm0'): 224, ('amd64', 'ymm1'): 256, ('amd64', 'ymm2'): 288, ('amd64', 'ymm3'): 320, ('amd64', 'ymm4'): 352, ('amd64', 'ymm5'): 384, ('amd64', 'ymm6'): 416, ('amd64', 'ymm7'): 448, ('amd64', 'ymm8'): 480, ('amd64', 'ymm9'): 512, ('amd64', 'ymm10'): 544, ('amd64', 'ymm11'): 576, ('amd64', 'ymm12'): 608, ('amd64', 'ymm13'): 640, ('amd64', 'ymm14'): 672, ('amd64', 'ymm15'): 704, ('amd64', 'ymm16'): 736, ('amd64', 'cr0'): 768, ('amd64', 'cr1'): 776, ('amd64', 'cr2'): 784, ('amd64', 'cr3'): 792, ('amd64', 'cr4'): 800, ('amd64', 'cr5'): 808, ('amd64', 'cr6'): 816, ('amd64', 'cr7'): 824, ('amd64', 'cr8'): 832, ('amd64', 'cr9'): 840, ('amd64', 'cr10'): 848, ('amd64', 'cr11'): 856, ('amd64', 'cr12'): 864, ('amd64', 'cr13'): 872, ('amd64', 'cr14'): 880, ('amd64', 'cr15'): 888, ('amd64', 'ftop'): 896, ('amd64', 'fpreg'): 904, ('amd64', 'fptag'): 968, ('amd64', 'fpround'): 976, ('amd64', 'fc3210'): 984, ('amd64', 'emnote'): 992, ('amd64', 'cmstart'): 1000, ('amd64', 'cmlen'): 1008, ('amd64', 'nraddr'): 1016, ('amd64', 'sc_class'): 1024, ('amd64', 'gs_const'): 1032, ('amd64', 'cs'): 1048, ('amd64', 'ds'): 1050, ('amd64', 'es'): 1052, ('amd64', 'fs'): 1054, ('amd64', 'gs'): 1056, ('amd64', 'ss'): 1058, ('ppc32', 'gpr0'): 16, ('ppc32', 'gpr1'): 20, ('ppc32', 'gpr2'): 24, ('ppc32', 'gpr3'): 28, ('ppc32', 'gpr4'): 32, ('ppc32', 'gpr5'): 36, ('ppc32', 'gpr6'): 40, ('ppc32', 'gpr7'): 44, ('ppc32', 'gpr8'): 48, ('ppc32', 'gpr9'): 52, ('ppc32', 'gpr10'): 56, ('ppc32', 'gpr11'): 60, ('ppc32', 'gpr12'): 64, ('ppc32', 'gpr13'): 68, ('ppc32', 'gpr14'): 72, ('ppc32', 'gpr15'): 76, ('ppc32', 'gpr16'): 80, ('ppc32', 'gpr17'): 84, ('ppc32', 'gpr18'): 88, ('ppc32', 'gpr19'): 92, ('ppc32', 'gpr20'): 96, ('ppc32', 'gpr21'): 100, ('ppc32', 'gpr22'): 104, ('ppc32', 'gpr23'): 108, ('ppc32', 'gpr24'): 112, ('ppc32', 'gpr25'): 116, ('ppc32', 'gpr26'): 120, ('ppc32', 'gpr27'): 124, ('ppc32', 'gpr28'): 128, ('ppc32', 'gpr29'): 132, ('ppc32', 'gpr30'): 136, ('ppc32', 'gpr31'): 140, ('ppc32', 'vsr0'): 144, ('ppc32', 'vsr1'): 160, ('ppc32', 'vsr2'): 176, ('ppc32', 'vsr3'): 192, ('ppc32', 'vsr4'): 208, ('ppc32', 'vsr5'): 224, ('ppc32', 'vsr6'): 240, ('ppc32', 'vsr7'): 256, ('ppc32', 'vsr8'): 272, ('ppc32', 'vsr9'): 288, ('ppc32', 'vsr10'): 304, ('ppc32', 'vsr11'): 320, ('ppc32', 'vsr12'): 336, ('ppc32', 'vsr13'): 352, ('ppc32', 'vsr14'): 368, ('ppc32', 'vsr15'): 384, ('ppc32', 'vsr16'): 400, ('ppc32', 'vsr17'): 416, ('ppc32', 'vsr18'): 432, ('ppc32', 'vsr19'): 448, ('ppc32', 'vsr20'): 464, ('ppc32', 'vsr21'): 480, ('ppc32', 'vsr22'): 496, ('ppc32', 'vsr23'): 512, ('ppc32', 'vsr24'): 528, ('ppc32', 'vsr25'): 544, ('ppc32', 'vsr26'): 560, ('ppc32', 'vsr27'): 576, ('ppc32', 'vsr28'): 592, ('ppc32', 'vsr29'): 608, ('ppc32', 'vsr30'): 624, ('ppc32', 'vsr31'): 640, ('ppc32', 'vsr32'): 656, ('ppc32', 'vsr33'): 672, ('ppc32', 'vsr34'): 688, ('ppc32', 'vsr35'): 704, ('ppc32', 'vsr36'): 720, ('ppc32', 'vsr37'): 736, ('ppc32', 'vsr38'): 752, ('ppc32', 'vsr39'): 768, ('ppc32', 'vsr40'): 784, ('ppc32', 'vsr41'): 800, ('ppc32', 'vsr42'): 816, ('ppc32', 'vsr43'): 832, ('ppc32', 'vsr44'): 848, ('ppc32', 'vsr45'): 864, ('ppc32', 'vsr46'): 880, ('ppc32', 'vsr47'): 896, ('ppc32', 'vsr48'): 912, ('ppc32', 'vsr49'): 928, ('ppc32', 'vsr50'): 944, ('ppc32', 'vsr51'): 960, ('ppc32', 'vsr52'): 976, ('ppc32', 'vsr53'): 992, ('ppc32', 'vsr54'): 1008, ('ppc32', 'vsr55'): 1024, ('ppc32', 'vsr56'): 1040, ('ppc32', 'vsr57'): 1056, ('ppc32', 'vsr58'): 1072, ('ppc32', 'vsr59'): 1088, ('ppc32', 'vsr60'): 1104, ('ppc32', 'vsr61'): 1120, ('ppc32', 'vsr62'): 1136, ('ppc32', 'vsr63'): 1152, ('ppc32', 'cia'): 1168, ('ppc32', 'lr'): 1172, ('ppc32', 'ctr'): 1176, ('ppc32', 'xer_so'): 1180, ('ppc32', 'xer_ov'): 1181, ('ppc32', 'xer_ca'): 1182, ('ppc32', 'xer_bc'): 1183, ('ppc32', 'cr0_321'): 1184, ('ppc32', 'cr0_0'): 1185, ('ppc32', 'cr1_321'): 1186, ('ppc32', 'cr1_0'): 1187, ('ppc32', 'cr2_321'): 1188, ('ppc32', 'cr2_0'): 1189, ('ppc32', 'cr3_321'): 1190, ('ppc32', 'cr3_0'): 1191, ('ppc32', 'cr4_321'): 1192, ('ppc32', 'cr4_0'): 1193, ('ppc32', 'cr5_321'): 1194, ('ppc32', 'cr5_0'): 1195, ('ppc32', 'cr6_321'): 1196, ('ppc32', 'cr6_0'): 1197, ('ppc32', 'cr7_321'): 1198, ('ppc32', 'cr7_0'): 1199, ('ppc32', 'fpround'): 1200, ('ppc32', 'dfpround'): 1201, ('ppc32', 'c_fpcc'): 1202, ('ppc32', 'vrsave'): 1204, ('ppc32', 'vscr'): 1208, ('ppc32', 'emnote'): 1212, ('ppc32', 'cmstart'): 1216, ('ppc32', 'cmlen'): 1220, ('ppc32', 'nraddr'): 1224, ('ppc32', 'nraddr_gpr2'): 1228, ('ppc32', 'redir_sp'): 1232, ('ppc32', 'redir_stack'): 1236, ('ppc32', 'ip_at_syscall'): 1364, ('ppc32', 'sprg3_ro'): 1368, ('ppc32', 'tfhar'): 1376, ('ppc32', 'texasr'): 1384, ('ppc32', 'tfiar'): 1392, ('ppc32', 'ppr'): 1400, ('ppc32', 'texasru'): 1408, ('ppc32', 'pspb'): 1412, ('ppc64', 'gpr0'): 16, ('ppc64', 'gpr1'): 24, ('ppc64', 'gpr2'): 32, ('ppc64', 'gpr3'): 40, ('ppc64', 'gpr4'): 48, ('ppc64', 'gpr5'): 56, ('ppc64', 'gpr6'): 64, ('ppc64', 'gpr7'): 72, ('ppc64', 'gpr8'): 80, ('ppc64', 'gpr9'): 88, ('ppc64', 'gpr10'): 96, ('ppc64', 'gpr11'): 104, ('ppc64', 'gpr12'): 112, ('ppc64', 'gpr13'): 120, ('ppc64', 'gpr14'): 128, ('ppc64', 'gpr15'): 136, ('ppc64', 'gpr16'): 144, ('ppc64', 'gpr17'): 152, ('ppc64', 'gpr18'): 160, ('ppc64', 'gpr19'): 168, ('ppc64', 'gpr20'): 176, ('ppc64', 'gpr21'): 184, ('ppc64', 'gpr22'): 192, ('ppc64', 'gpr23'): 200, ('ppc64', 'gpr24'): 208, ('ppc64', 'gpr25'): 216, ('ppc64', 'gpr26'): 224, ('ppc64', 'gpr27'): 232, ('ppc64', 'gpr28'): 240, ('ppc64', 'gpr29'): 248, ('ppc64', 'gpr30'): 256, ('ppc64', 'gpr31'): 264, ('ppc64', 'vsr0'): 272, ('ppc64', 'vsr1'): 288, ('ppc64', 'vsr2'): 304, ('ppc64', 'vsr3'): 320, ('ppc64', 'vsr4'): 336, ('ppc64', 'vsr5'): 352, ('ppc64', 'vsr6'): 368, ('ppc64', 'vsr7'): 384, ('ppc64', 'vsr8'): 400, ('ppc64', 'vsr9'): 416, ('ppc64', 'vsr10'): 432, ('ppc64', 'vsr11'): 448, ('ppc64', 'vsr12'): 464, ('ppc64', 'vsr13'): 480, ('ppc64', 'vsr14'): 496, ('ppc64', 'vsr15'): 512, ('ppc64', 'vsr16'): 528, ('ppc64', 'vsr17'): 544, ('ppc64', 'vsr18'): 560, ('ppc64', 'vsr19'): 576, ('ppc64', 'vsr20'): 592, ('ppc64', 'vsr21'): 608, ('ppc64', 'vsr22'): 624, ('ppc64', 'vsr23'): 640, ('ppc64', 'vsr24'): 656, ('ppc64', 'vsr25'): 672, ('ppc64', 'vsr26'): 688, ('ppc64', 'vsr27'): 704, ('ppc64', 'vsr28'): 720, ('ppc64', 'vsr29'): 736, ('ppc64', 'vsr30'): 752, ('ppc64', 'vsr31'): 768, ('ppc64', 'vsr32'): 784, ('ppc64', 'vsr33'): 800, ('ppc64', 'vsr34'): 816, ('ppc64', 'vsr35'): 832, ('ppc64', 'vsr36'): 848, ('ppc64', 'vsr37'): 864, ('ppc64', 'vsr38'): 880, ('ppc64', 'vsr39'): 896, ('ppc64', 'vsr40'): 912, ('ppc64', 'vsr41'): 928, ('ppc64', 'vsr42'): 944, ('ppc64', 'vsr43'): 960, ('ppc64', 'vsr44'): 976, ('ppc64', 'vsr45'): 992, ('ppc64', 'vsr46'): 1008, ('ppc64', 'vsr47'): 1024, ('ppc64', 'vsr48'): 1040, ('ppc64', 'vsr49'): 1056, ('ppc64', 'vsr50'): 1072, ('ppc64', 'vsr51'): 1088, ('ppc64', 'vsr52'): 1104, ('ppc64', 'vsr53'): 1120, ('ppc64', 'vsr54'): 1136, ('ppc64', 'vsr55'): 1152, ('ppc64', 'vsr56'): 1168, ('ppc64', 'vsr57'): 1184, ('ppc64', 'vsr58'): 1200, ('ppc64', 'vsr59'): 1216, ('ppc64', 'vsr60'): 1232, ('ppc64', 'vsr61'): 1248, ('ppc64', 'vsr62'): 1264, ('ppc64', 'vsr63'): 1280, ('ppc64', 'cia'): 1296, ('ppc64', 'lr'): 1304, ('ppc64', 'ctr'): 1312, ('ppc64', 'xer_so'): 1320, ('ppc64', 'xer_ov'): 1321, ('ppc64', 'xer_ca'): 1322, ('ppc64', 'xer_bc'): 1323, ('ppc64', 'cr0_321'): 1324, ('ppc64', 'cr0_0'): 1325, ('ppc64', 'cr1_321'): 1326, ('ppc64', 'cr1_0'): 1327, ('ppc64', 'cr2_321'): 1328, ('ppc64', 'cr2_0'): 1329, ('ppc64', 'cr3_321'): 1330, ('ppc64', 'cr3_0'): 1331, ('ppc64', 'cr4_321'): 1332, ('ppc64', 'cr4_0'): 1333, ('ppc64', 'cr5_321'): 1334, ('ppc64', 'cr5_0'): 1335, ('ppc64', 'cr6_321'): 1336, ('ppc64', 'cr6_0'): 1337, ('ppc64', 'cr7_321'): 1338, ('ppc64', 'cr7_0'): 1339, ('ppc64', 'fpround'): 1340, ('ppc64', 'dfpround'): 1341, ('ppc64', 'c_fpcc'): 1342, ('ppc64', 'vrsave'): 1344, ('ppc64', 'vscr'): 1348, ('ppc64', 'emnote'): 1352, ('ppc64', 'cmstart'): 1360, ('ppc64', 'cmlen'): 1368, ('ppc64', 'nraddr'): 1376, ('ppc64', 'nraddr_gpr2'): 1384, ('ppc64', 'redir_sp'): 1392, ('ppc64', 'redir_stack'): 1400, ('ppc64', 'ip_at_syscall'): 1656, ('ppc64', 'sprg3_ro'): 1664, ('ppc64', 'tfhar'): 1672, ('ppc64', 'texasr'): 1680, ('ppc64', 'tfiar'): 1688, ('ppc64', 'ppr'): 1696, ('ppc64', 'texasru'): 1704, ('ppc64', 'pspb'): 1708, ('arm', 'r0'): 8, ('arm', 'r1'): 12, ('arm', 'r2'): 16, ('arm', 'r3'): 20, ('arm', 'r4'): 24, ('arm', 'r5'): 28, ('arm', 'r6'): 32, ('arm', 'r7'): 36, ('arm', 'r8'): 40, ('arm', 'r9'): 44, ('arm', 'r10'): 48, ('arm', 'r11'): 52, ('arm', 'r12'): 56, ('arm', 'r13'): 60, ('arm', 'r14'): 64, ('arm', 'r15t'): 68, ('arm', 'cc_op'): 72, ('arm', 'cc_dep1'): 76, ('arm', 'cc_dep2'): 80, ('arm', 'cc_ndep'): 84, ('arm', 'qflag32'): 88, ('arm', 'geflag0'): 92, ('arm', 'geflag1'): 96, ('arm', 'geflag2'): 100, ('arm', 'geflag3'): 104, ('arm', 'emnote'): 108, ('arm', 'cmstart'): 112, ('arm', 'cmlen'): 116, ('arm', 'nraddr'): 120, ('arm', 'ip_at_syscall'): 124, ('arm', 'd0'): 128, ('arm', 'd1'): 136, ('arm', 'd2'): 144, ('arm', 'd3'): 152, ('arm', 'd4'): 160, ('arm', 'd5'): 168, ('arm', 'd6'): 176, ('arm', 'd7'): 184, ('arm', 'd8'): 192, ('arm', 'd9'): 200, ('arm', 'd10'): 208, ('arm', 'd11'): 216, ('arm', 'd12'): 224, ('arm', 'd13'): 232, ('arm', 'd14'): 240, ('arm', 'd15'): 248, ('arm', 'd16'): 256, ('arm', 'd17'): 264, ('arm', 'd18'): 272, ('arm', 'd19'): 280, ('arm', 'd20'): 288, ('arm', 'd21'): 296, ('arm', 'd22'): 304, ('arm', 'd23'): 312, ('arm', 'd24'): 320, ('arm', 'd25'): 328, ('arm', 'd26'): 336, ('arm', 'd27'): 344, ('arm', 'd28'): 352, ('arm', 'd29'): 360, ('arm', 'd30'): 368, ('arm', 'd31'): 376, ('arm', 'fpscr'): 384, ('arm', 'tpidruro'): 388, ('arm', 'itstate'): 392, ('arm64', 'x0'): 16, ('arm64', 'x1'): 24, ('arm64', 'x2'): 32, ('arm64', 'x3'): 40, ('arm64', 'x4'): 48, ('arm64', 'x5'): 56, ('arm64', 'x6'): 64, ('arm64', 'x7'): 72, ('arm64', 'x8'): 80, ('arm64', 'x9'): 88, ('arm64', 'x10'): 96, ('arm64', 'x11'): 104, ('arm64', 'x12'): 112, ('arm64', 'x13'): 120, ('arm64', 'x14'): 128, ('arm64', 'x15'): 136, ('arm64', 'x16'): 144, ('arm64', 'x17'): 152, ('arm64', 'x18'): 160, ('arm64', 'x19'): 168, ('arm64', 'x20'): 176, ('arm64', 'x21'): 184, ('arm64', 'x22'): 192, ('arm64', 'x23'): 200, ('arm64', 'x24'): 208, ('arm64', 'x25'): 216, ('arm64', 'x26'): 224, ('arm64', 'x27'): 232, ('arm64', 'x28'): 240, ('arm64', 'x29'): 248, ('arm64', 'x30'): 256, ('arm64', 'xsp'): 264, ('arm64', 'pc'): 272, ('arm64', 'cc_op'): 280, ('arm64', 'cc_dep1'): 288, ('arm64', 'cc_dep2'): 296, ('arm64', 'cc_ndep'): 304, ('arm64', 'tpidr_el0'): 312, ('arm64', 'q0'): 320, ('arm64', 'q1'): 336, ('arm64', 'q2'): 352, ('arm64', 'q3'): 368, ('arm64', 'q4'): 384, ('arm64', 'q5'): 400, ('arm64', 'q6'): 416, ('arm64', 'q7'): 432, ('arm64', 'q8'): 448, ('arm64', 'q9'): 464, ('arm64', 'q10'): 480, ('arm64', 'q11'): 496, ('arm64', 'q12'): 512, ('arm64', 'q13'): 528, ('arm64', 'q14'): 544, ('arm64', 'q15'): 560, ('arm64', 'q16'): 576, ('arm64', 'q17'): 592, ('arm64', 'q18'): 608, ('arm64', 'q19'): 624, ('arm64', 'q20'): 640, ('arm64', 'q21'): 656, ('arm64', 'q22'): 672, ('arm64', 'q23'): 688, ('arm64', 'q24'): 704, ('arm64', 'q25'): 720, ('arm64', 'q26'): 736, ('arm64', 'q27'): 752, ('arm64', 'q28'): 768, ('arm64', 'q29'): 784, ('arm64', 'q30'): 800, ('arm64', 'q31'): 816, ('arm64', 'qcflag'): 832, ('arm64', 'emnote'): 848, ('arm64', 'cmstart'): 856, ('arm64', 'cmlen'): 864, ('arm64', 'nraddr'): 872, ('arm64', 'ip_at_syscall'): 880, ('arm64', 'fpcr'): 888, ('s390x', 'a0'): 0, ('s390x', 'a1'): 4, ('s390x', 'a2'): 8, ('s390x', 'a3'): 12, ('s390x', 'a4'): 16, ('s390x', 'a5'): 20, ('s390x', 'a6'): 24, ('s390x', 'a7'): 28, ('s390x', 'a8'): 32, ('s390x', 'a9'): 36, ('s390x', 'a10'): 40, ('s390x', 'a11'): 44, ('s390x', 'a12'): 48, ('s390x', 'a13'): 52, ('s390x', 'a14'): 56, ('s390x', 'a15'): 60, ('s390x', 'v0'): 64, ('s390x', 'v1'): 80, ('s390x', 'v2'): 96, ('s390x', 'v3'): 112, ('s390x', 'v4'): 128, ('s390x', 'v5'): 144, ('s390x', 'v6'): 160, ('s390x', 'v7'): 176, ('s390x', 'v8'): 192, ('s390x', 'v9'): 208, ('s390x', 'v10'): 224, ('s390x', 'v11'): 240, ('s390x', 'v12'): 256, ('s390x', 'v13'): 272, ('s390x', 'v14'): 288, ('s390x', 'v15'): 304, ('s390x', 'v16'): 320, ('s390x', 'v17'): 336, ('s390x', 'v18'): 352, ('s390x', 'v19'): 368, ('s390x', 'v20'): 384, ('s390x', 'v21'): 400, ('s390x', 'v22'): 416, ('s390x', 'v23'): 432, ('s390x', 'v24'): 448, ('s390x', 'v25'): 464, ('s390x', 'v26'): 480, ('s390x', 'v27'): 496, ('s390x', 'v28'): 512, ('s390x', 'v29'): 528, ('s390x', 'v30'): 544, ('s390x', 'v31'): 560, ('s390x', 'r0'): 576, ('s390x', 'r1'): 584, ('s390x', 'r2'): 592, ('s390x', 'r3'): 600, ('s390x', 'r4'): 608, ('s390x', 'r5'): 616, ('s390x', 'r6'): 624, ('s390x', 'r7'): 632, ('s390x', 'r8'): 640, ('s390x', 'r9'): 648, ('s390x', 'r10'): 656, ('s390x', 'r11'): 664, ('s390x', 'r12'): 672, ('s390x', 'r13'): 680, ('s390x', 'r14'): 688, ('s390x', 'r15'): 696, ('s390x', 'counter'): 704, ('s390x', 'fpc'): 712, ('s390x', 'ia'): 720, ('s390x', 'sysno'): 728, ('s390x', 'cc_op'): 736, ('s390x', 'cc_dep1'): 744, ('s390x', 'cc_dep2'): 752, ('s390x', 'cc_ndep'): 760, ('s390x', 'nraddr'): 768, ('s390x', 'cmstart'): 776, ('s390x', 'cmlen'): 784, ('s390x', 'ip_at_syscall'): 792, ('s390x', 'emnote'): 800, ('mips32', 'r0'): 8, ('mips32', 'r1'): 12, ('mips32', 'r2'): 16, ('mips32', 'r3'): 20, ('mips32', 'r4'): 24, ('mips32', 'r5'): 28, ('mips32', 'r6'): 32, ('mips32', 'r7'): 36, ('mips32', 'r8'): 40, ('mips32', 'r9'): 44, ('mips32', 'r10'): 48, ('mips32', 'r11'): 52, ('mips32', 'r12'): 56, ('mips32', 'r13'): 60, ('mips32', 'r14'): 64, ('mips32', 'r15'): 68, ('mips32', 'r16'): 72, ('mips32', 'r17'): 76, ('mips32', 'r18'): 80, ('mips32', 'r19'): 84, ('mips32', 'r20'): 88, ('mips32', 'r21'): 92, ('mips32', 'r22'): 96, ('mips32', 'r23'): 100, ('mips32', 'r24'): 104, ('mips32', 'r25'): 108, ('mips32', 'r26'): 112, ('mips32', 'r27'): 116, ('mips32', 'r28'): 120, ('mips32', 'r29'): 124, ('mips32', 'r30'): 128, ('mips32', 'r31'): 132, ('mips32', 'pc'): 136, ('mips32', 'hi'): 140, ('mips32', 'lo'): 144, ('mips32', 'f0'): 152, ('mips32', 'f1'): 160, ('mips32', 'f2'): 168, ('mips32', 'f3'): 176, ('mips32', 'f4'): 184, ('mips32', 'f5'): 192, ('mips32', 'f6'): 200, ('mips32', 'f7'): 208, ('mips32', 'f8'): 216, ('mips32', 'f9'): 224, ('mips32', 'f10'): 232, ('mips32', 'f11'): 240, ('mips32', 'f12'): 248, ('mips32', 'f13'): 256, ('mips32', 'f14'): 264, ('mips32', 'f15'): 272, ('mips32', 'f16'): 280, ('mips32', 'f17'): 288, ('mips32', 'f18'): 296, ('mips32', 'f19'): 304, ('mips32', 'f20'): 312, ('mips32', 'f21'): 320, ('mips32', 'f22'): 328, ('mips32', 'f23'): 336, ('mips32', 'f24'): 344, ('mips32', 'f25'): 352, ('mips32', 'f26'): 360, ('mips32', 'f27'): 368, ('mips32', 'f28'): 376, ('mips32', 'f29'): 384, ('mips32', 'f30'): 392, ('mips32', 'f31'): 400, ('mips32', 'fir'): 408, ('mips32', 'fccr'): 412, ('mips32', 'fexr'): 416, ('mips32', 'fenr'): 420, ('mips32', 'fcsr'): 424, ('mips32', 'ulr'): 428, ('mips32', 'emnote'): 432, ('mips32', 'cmstart'): 436, ('mips32', 'cmlen'): 440, ('mips32', 'nraddr'): 444, ('mips32', 'cond'): 448, ('mips32', 'dspcontrol'): 452, ('mips32', 'ac0'): 456, ('mips32', 'ac1'): 464, ('mips32', 'ac2'): 472, ('mips32', 'ac3'): 480, ('mips32', 'cp0_status'): 488, ('mips32', 'ip_at_syscall'): 492, ('mips64', 'r0'): 16, ('mips64', 'r1'): 24, ('mips64', 'r2'): 32, ('mips64', 'r3'): 40, ('mips64', 'r4'): 48, ('mips64', 'r5'): 56, ('mips64', 'r6'): 64, ('mips64', 'r7'): 72, ('mips64', 'r8'): 80, ('mips64', 'r9'): 88, ('mips64', 'r10'): 96, ('mips64', 'r11'): 104, ('mips64', 'r12'): 112, ('mips64', 'r13'): 120, ('mips64', 'r14'): 128, ('mips64', 'r15'): 136, ('mips64', 'r16'): 144, ('mips64', 'r17'): 152, ('mips64', 'r18'): 160, ('mips64', 'r19'): 168, ('mips64', 'r20'): 176, ('mips64', 'r21'): 184, ('mips64', 'r22'): 192, ('mips64', 'r23'): 200, ('mips64', 'r24'): 208, ('mips64', 'r25'): 216, ('mips64', 'r26'): 224, ('mips64', 'r27'): 232, ('mips64', 'r28'): 240, ('mips64', 'r29'): 248, ('mips64', 'r30'): 256, ('mips64', 'r31'): 264, ('mips64', 'pc'): 272, ('mips64', 'hi'): 280, ('mips64', 'lo'): 288, ('mips64', 'f0'): 296, ('mips64', 'f1'): 304, ('mips64', 'f2'): 312, ('mips64', 'f3'): 320, ('mips64', 'f4'): 328, ('mips64', 'f5'): 336, ('mips64', 'f6'): 344, ('mips64', 'f7'): 352, ('mips64', 'f8'): 360, ('mips64', 'f9'): 368, ('mips64', 'f10'): 376, ('mips64', 'f11'): 384, ('mips64', 'f12'): 392, ('mips64', 'f13'): 400, ('mips64', 'f14'): 408, ('mips64', 'f15'): 416, ('mips64', 'f16'): 424, ('mips64', 'f17'): 432, ('mips64', 'f18'): 440, ('mips64', 'f19'): 448, ('mips64', 'f20'): 456, ('mips64', 'f21'): 464, ('mips64', 'f22'): 472, ('mips64', 'f23'): 480, ('mips64', 'f24'): 488, ('mips64', 'f25'): 496, ('mips64', 'f26'): 504, ('mips64', 'f27'): 512, ('mips64', 'f28'): 520, ('mips64', 'f29'): 528, ('mips64', 'f30'): 536, ('mips64', 'f31'): 544, ('mips64', 'fir'): 552, ('mips64', 'fccr'): 556, ('mips64', 'fexr'): 560, ('mips64', 'fenr'): 564, ('mips64', 'fcsr'): 568, ('mips64', 'cp0_status'): 572, ('mips64', 'ulr'): 576, ('mips64', 'emnote'): 584, ('mips64', 'cond'): 588, ('mips64', 'cmstart'): 592, ('mips64', 'cmlen'): 600, ('mips64', 'nraddr'): 608, ('mips64', 'ip_at_syscall'): 616, ('tilegx', 'r0'): 0, ('tilegx', 'r1'): 8, ('tilegx', 'r2'): 16, ('tilegx', 'r3'): 24, ('tilegx', 'r4'): 32, ('tilegx', 'r5'): 40, ('tilegx', 'r6'): 48, ('tilegx', 'r7'): 56, ('tilegx', 'r8'): 64, ('tilegx', 'r9'): 72, ('tilegx', 'r10'): 80, ('tilegx', 'r11'): 88, ('tilegx', 'r12'): 96, ('tilegx', 'r13'): 104, ('tilegx', 'r14'): 112, ('tilegx', 'r15'): 120, ('tilegx', 'r16'): 128, ('tilegx', 'r17'): 136, ('tilegx', 'r18'): 144, ('tilegx', 'r19'): 152, ('tilegx', 'r20'): 160, ('tilegx', 'r21'): 168, ('tilegx', 'r22'): 176, ('tilegx', 'r23'): 184, ('tilegx', 'r24'): 192, ('tilegx', 'r25'): 200, ('tilegx', 'r26'): 208, ('tilegx', 'r27'): 216, ('tilegx', 'r28'): 224, ('tilegx', 'r29'): 232, ('tilegx', 'r30'): 240, ('tilegx', 'r31'): 248, ('tilegx', 'r32'): 256, ('tilegx', 'r33'): 264, ('tilegx', 'r34'): 272, ('tilegx', 'r35'): 280, ('tilegx', 'r36'): 288, ('tilegx', 'r37'): 296, ('tilegx', 'r38'): 304, ('tilegx', 'r39'): 312, ('tilegx', 'r40'): 320, ('tilegx', 'r41'): 328, ('tilegx', 'r42'): 336, ('tilegx', 'r43'): 344, ('tilegx', 'r44'): 352, ('tilegx', 'r45'): 360, ('tilegx', 'r46'): 368, ('tilegx', 'r47'): 376, ('tilegx', 'r48'): 384, ('tilegx', 'r49'): 392, ('tilegx', 'r50'): 400, ('tilegx', 'r51'): 408, ('tilegx', 'r52'): 416, ('tilegx', 'r53'): 424, ('tilegx', 'r54'): 432, ('tilegx', 'r55'): 440, ('tilegx', 'r56'): 448, ('tilegx', 'r57'): 456, ('tilegx', 'r58'): 464, ('tilegx', 'r59'): 472, ('tilegx', 'r60'): 480, ('tilegx', 'r61'): 488, ('tilegx', 'r62'): 496, ('tilegx', 'r63'): 504, ('tilegx', 'pc'): 512, ('tilegx', 'spare'): 520, ('tilegx', 'emnote'): 528, ('tilegx', 'cmstart'): 536, ('tilegx', 'cmlen'): 544, ('tilegx', 'nraddr'): 552, ('tilegx', 'cmpexch'): 560, ('tilegx', 'zero'): 568, ('tilegx', 'ex_context_0'): 576, ('tilegx', 'ex_context_1'): 584, ('tilegx', 'cond'): 608, ('riscv64', 'x0'): 16, ('riscv64', 'x1'): 24, ('riscv64', 'x2'): 32, ('riscv64', 'x3'): 40, ('riscv64', 'x4'): 48, ('riscv64', 'x5'): 56, ('riscv64', 'x6'): 64, ('riscv64', 'x7'): 72, ('riscv64', 'x9'): 88, ('riscv64', 'x10'): 96, ('riscv64', 'x11'): 104, ('riscv64', 'x12'): 112, ('riscv64', 'x13'): 120, ('riscv64', 'x14'): 128, ('riscv64', 'x15'): 136, ('riscv64', 'x16'): 144, ('riscv64', 'x17'): 152, ('riscv64', 'x18'): 160, ('riscv64', 'x19'): 168, ('riscv64', 'x20'): 176, ('riscv64', 'x21'): 184, ('riscv64', 'x22'): 192, ('riscv64', 'x23'): 200, ('riscv64', 'x24'): 208, ('riscv64', 'x25'): 216, ('riscv64', 'x26'): 224, ('riscv64', 'x27'): 232, ('riscv64', 'x28'): 240, ('riscv64', 'x29'): 248, ('riscv64', 'x30'): 256, ('riscv64', 'x31'): 264, ('riscv64', 'pc'): 272, ('riscv64', 'f0'): 280, ('riscv64', 'f1'): 288, ('riscv64', 'f2'): 296, ('riscv64', 'f3'): 304, ('riscv64', 'f4'): 312, ('riscv64', 'f5'): 320, ('riscv64', 'f6'): 328, ('riscv64', 'f7'): 336, ('riscv64', 'f9'): 352, ('riscv64', 'f10'): 360, ('riscv64', 'f11'): 368, ('riscv64', 'f12'): 376, ('riscv64', 'f13'): 384, ('riscv64', 'f14'): 392, ('riscv64', 'f15'): 400, ('riscv64', 'f16'): 408, ('riscv64', 'f17'): 416, ('riscv64', 'f18'): 424, ('riscv64', 'f19'): 432, ('riscv64', 'f20'): 440, ('riscv64', 'f21'): 448, ('riscv64', 'f22'): 456, ('riscv64', 'f23'): 464, ('riscv64', 'f24'): 472, ('riscv64', 'f25'): 480, ('riscv64', 'f26'): 488, ('riscv64', 'f27'): 496, ('riscv64', 'f28'): 504, ('riscv64', 'f29'): 512, ('riscv64', 'f30'): 520, ('riscv64', 'f31'): 528}
