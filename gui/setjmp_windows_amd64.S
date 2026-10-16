// __intrinsic_setjmpex(jmp_buf, frame) is _setjmpex(jmp_buf, frame) of msvcrt. It is a jump,
// not a call, so _setjmpex saves the frame of the caller and returns straight to it.
	.text
	.globl	__intrinsic_setjmpex
__intrinsic_setjmpex:
	jmp	_setjmpex
