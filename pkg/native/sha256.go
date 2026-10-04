package native

// Phase 152-B2: karkain_sha256_hex, the native implementation of the existing
// `sha256_hex` builtin.
//
// The C23 reference is `karkain_sha256` in pkg/codegen/codegen.go and remains
// the behavioural authority: this mirrors it byte for byte, including its
// `strlen` contract (the digest stops at the first NUL) and its lowercase hex
// output. There is deliberately no length-aware variant.
//
// Per docs/audit/PHASE-152-BASELINE.md §11.1.1 this adds NO language-level
// bitwise operator. The emitter primitives it needs were already committed for
// this work: NotReg (F7 /2) and XorRegReg2 (31 /r, two operands). NOTE that the
// pre-existing XorRegReg is the ONE-operand `xor r, r` zeroing form and
// DESTROYS its operand -- it must never be used here.
//
// 32-bit discipline: every operation runs on a 64-bit unit whose upper 32 bits
// are kept zero. AND/OR/XOR cannot carry. Addition wraps at 2^64, so the low 32
// bits are already correct and a mask only clears the carry. Shifts are logical
// and masked. Rotate needs the zero-extension identity
//
//	ROL32(v,n) = (ROL64(v,n) & 0xFFFFFFFF) | (ROL64(v,n) >> 32)
//
// because RolRegImm is REX.W and so rotates 64 bits.

// sha256IV is the SHA-256 initial hash value (FIPS 180-4 §5.3.3).
var sha256IV = [8]uint32{
	0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
	0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
}

// sha256KWords is the round-constant table (FIPS 180-4 §4.2.2).
//
// Verified against the FIPS 180-4 DEFINITION (fractional parts of the cube roots
// of the first 64 primes) by TestPhase152B2_KTableMatchesFIPS, so a mistyped
// constant cannot reach an emitted image: an earlier draft carried 0x80deb01f
// where the table says 0x80deb1fe. Measured: the digest vectors catch that too,
// so this gate's contribution is naming WHICH word is wrong, not that one is.
var sha256KWords = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5,
	0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3,
	0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc,
	0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
	0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13,
	0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3,
	0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5,
	0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208,
	0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

// sha256K packs the table little-endian so the helper reads it with the same
// width it was written with (the message, by contrast, is big-endian and is
// assembled byte by byte).
var sha256K = func() string {
	var b [256]byte
	for i, w := range sha256KWords {
		b[i*4+0] = byte(w)
		b[i*4+1] = byte(w >> 8)
		b[i*4+2] = byte(w >> 16)
		b[i*4+3] = byte(w >> 24)
	}
	return string(b[:])
}()

// sha256HexDigits is the lowercase alphabet P109_HEXD indexes.
const sha256HexDigits = "0123456789abcdef"

// sha256Frame is the helper's frame layout, as named constants rather than bare
// numbers so an overlap is a compile error instead of a silent corruption.
//
// The FIRST draft of this helper declared offH = 88 and offW = 120 in one const
// block: H[8] occupies 88..151, so offW landed 32 bytes INSIDE the hash state and
// the first message-schedule store silently overwrote H[4..7]. All four harness
// vectors failed and the fault was in the layout, not in any algorithm. The
// regions below are therefore ordered and sized explicitly.
const (
	shaOffIn     = 0
	shaOffN      = 8
	shaOffFile   = 16
	shaOffFileN  = 24
	shaOffLine   = 32
	shaOffOut    = 40
	shaOffMsgLen = 48
	shaOffPadLen = 56
	shaOffTotal  = 64
	shaOffPos    = 72
	shaOffK      = 80
	shaOffH      = 88  // 8 units -> 88..151
	shaOffW      = 152 // 64 units -> 152..663
	shaOffPad    = 664 // at most 64+8 bytes -> 664..735
	shaOffV      = 736 // working a..h, 8 units -> 736..799
	shaOffTab    = 800 // 16 bytes -> 800..815
	shaOffO      = 816 // hex output cursor
	shaOffT      = 824 // round counter
	shaOffT1     = 832
	shaOffT2     = 840
	// A helper is entered with rsp == 8 (mod 16) after the call pushed the
	// return address, so a CALLING helper needs a frame of 8 (mod 16) to leave
	// rsp 16-byte aligned at its outgoing `call alloc`. 856 = 8 (mod 16) and
	// covers the highest slot (shaOffT2 = 840).
	shaFrameBytes = 856
)

// emitSHA256Helper emits karkain_sha256_hex.
//
//	karkain_sha256_hex(ptr, len, filePtr, fileLen, line) -> (RAX=ptr, RDX=64)
//
// The caller stages the argument in RDI/RSI and the diagnostic units in
// RDX/RCX/R8, then moves the returned (RAX, RDX) pair into (RDI, RSI). None of
// the diagnostic units are consumed: SHA-256 cannot fail. They are accepted so
// the shared emitStringCodec call site needs no special case.
func (b *Builder) emitSHA256Helper() {
	e := b.e
	e.Mark("karkain_sha256_hex")
	e.SubRsp(shaFrameBytes)
	e.StoreStack(RDI, shaOffIn)
	e.StoreStack(RSI, shaOffN)
	e.StoreStack(RDX, shaOffFile)
	e.StoreStack(RCX, shaOffFileN)
	e.StoreStack(R8, shaOffLine)

	// Only caller-saved registers are used (RAX RCX RDX RSI RDI R8-R11), so
	// there is no push/pop anywhere in this helper and rsp stays put.

	// --- strlen: truncate at the first NUL, exactly as the C23 reference does.
	// A native string carries an explicit length, so without this scan a value
	// containing a NUL would hash MORE bytes than strlen would and the two
	// digests would silently diverge.
	strLbl := b.fresh("sha$strlen")
	strDone := b.fresh("sha$strlen$done")
	e.XorRegReg(RAX)
	e.StoreStack(RAX, shaOffMsgLen)
	e.Mark(strLbl)
	e.LoadStack(RAX, shaOffMsgLen)
	e.LoadStack(RCX, shaOffN)
	e.CmpRegReg(RAX, RCX)
	e.Jae(strDone)
	e.LoadStack(RSI, shaOffIn)
	e.LoadScaled8(RDX, RSI, RAX, 1, 0)
	e.TestRegReg(RDX, RDX)
	e.Jz(strDone)
	e.IncReg(RAX)
	e.StoreStack(RAX, shaOffMsgLen)
	e.Jmp(strLbl)
	e.Mark(strDone)

	// out = alloc(65): 64 hex digits plus the NUL the reference writes.
	e.MovRegImm32(RDI, 65)
	e.Call("alloc")
	e.StoreStack(RAX, shaOffOut)

	b.rodataRef(RAX, sha256K)
	e.StoreStack(RAX, shaOffK)
	b.rodataRef(RAX, sha256HexDigits)
	e.StoreStack(RAX, shaOffTab)

	// --- padlen and total ---
	// C: padlen = ((len + 9 + 63) / 64) * 64 - len, and total = len + padlen,
	// which collapses to ((len+72)/64)*64.
	e.LoadStack(RAX, shaOffMsgLen)
	e.AddRegImm32(RAX, 72)
	e.ShrRegImm(RAX, 6)
	e.ShlRegImm(RAX, 6)
	e.StoreStack(RAX, shaOffTotal)
	e.LoadStack(RCX, shaOffMsgLen)
	e.SubRegReg(RAX, RCX)
	e.StoreStack(RAX, shaOffPadLen)

	// --- zero pad[0..padlen) ---
	e.LoadStack(RAX, shaOffPadLen)
	e.XorRegReg(RCX)
	e.XorRegReg(RDX)
	pz := b.fresh("sha$padz")
	pzd := b.fresh("sha$padzd")
	e.Mark(pz)
	e.CmpRegReg(RCX, RAX)
	e.Jae(pzd)
	e.StoreScaled8(RDX, RSP, RCX, 1, shaOffPad)
	e.IncReg(RCX)
	e.Jmp(pz)
	e.Mark(pzd)

	// pad[0] = 0x80
	e.MovRegImm32(RAX, 0x80)
	e.StoreMem8Off(RAX, RSP, shaOffPad)

	// pad[padlen-8 .. padlen-1] = big-endian bit length. Unrolled because the
	// shift must be an immediate; a shift-by-CL primitive would be another new
	// emitter primitive and §11.1.1 avoids that.
	e.LoadStack(RSI, shaOffPadLen)
	e.SubRegImm32(RSI, 8)
	for j := 0; j < 8; j++ {
		e.LoadStack(RAX, shaOffMsgLen)
		e.ShlRegImm(RAX, 3)
		if s := byte(56 - 8*j); s > 0 {
			e.ShrRegImm(RAX, s)
		}
		e.MovRegImm32(RCX, 0xFF)
		e.AndRegReg(RAX, RCX)
		e.StoreScaled8(RAX, RSP, RSI, 1, shaOffPad+j)
	}

	// --- initial hash value ---
	for i := 0; i < 8; i++ {
		e.MovRegImm64(RAX, uint64(sha256IV[i]))
		e.StoreStack(RAX, shaOffH+i*8)
	}
	// ------------------------------------------------------------------
	// Stage 2: the message schedule W[0..63].
	//
	// W[0..15] are the padded message read as big-endian 32-bit words.
	// W[16..63] follow FIPS 180-4 §4.1.2:
	//
	//	W[t] = s1(W[t-2]) + W[t-7] + s0(W[t-15]) + W[t-16]
	//	s0(x) = ROTR7(x)  ^ ROTR18(x) ^ SHR3(x)
	//	s1(x) = ROTR17(x) ^ ROTR19(x) ^ SHR10(x)
	//
	// ROTR(n) is emitted as ROL(32-n) because RolRegImm is REX.W and so
	// rotates 64 bits.
	// ------------------------------------------------------------------

	// rol32 emits ROL32(src -> dst, n). RolRegImm rotates 64 bits, so the
	// zero-extension identity is used: with v's upper 32 bits zero,
	// ROL64(v,n) puts the wrapped bits at 32..31+n and the plain shift below
	// that, so ((v<<n) & 0xFFFFFFFF) | ((v<<n) >> 32) is exactly ROL32.
	rol32 := func(src, dst, scr Reg, n byte) {
		e.MovRegReg(dst, src)
		e.RolRegImm(dst, n)
		e.MovRegReg(scr, dst)
		e.ShrRegImm(scr, 32)
		e.MovRegImm32(RCX, 0xFFFFFFFF)
		e.AndRegReg(dst, RCX)
		e.OrRegReg(dst, scr)
	}

	// mask32 clears everything above bit 31.
	mask32 := func(r Reg) {
		e.MovRegImm32(RCX, 0xFFFFFFFF)
		e.AndRegReg(r, RCX)
	}

	// s0of: R8 holds x, result lands in R9. Register plan keeps R9 (the
	// accumulator) free while the ROL scratch RSI and mask RCX are used.
	s0of := func() {
		rol32(R8, RAX, RSI, 25) // ROTR7
		rol32(R8, RDX, RSI, 14) // ROTR18
		e.MovRegReg(RDI, R8)
		e.ShrRegImm(RDI, 3)
		mask32(RDI)
		e.MovRegReg(R9, RAX)
		e.XorRegReg2(R9, RDX)
		e.XorRegReg2(R9, RDI)
	}

	// s1of: R8 holds x, result lands in R10 (deliberately NOT R9).
	s1of := func() {
		rol32(R8, RAX, RSI, 15) // ROTR17
		rol32(R8, RDX, RSI, 13) // ROTR19
		e.MovRegReg(RDI, R8)
		e.ShrRegImm(RDI, 10)
		mask32(RDI)
		e.MovRegReg(R10, RAX)
		e.XorRegReg2(R10, RDX)
		e.XorRegReg2(R10, RDI)
	}

	// byteAt emits "RDX = the SHA-256 source byte at index RAX", mirroring the
	// C reference's src < len ? msg[src] : pad[src-len]. Labels are fresh per
	// invocation, and it is emitted once inside the loop body below.
	byteAt := func() {
		inPath := b.fresh("sha$in")
		atDone := b.fresh("sha$at")
		e.LoadStack(RCX, shaOffMsgLen)
		e.CmpRegReg(RAX, RCX)
		e.Jb(inPath)
		e.SubRegReg(RAX, RCX)
		e.LoadScaled8(RDX, RSP, RAX, 1, shaOffPad)
		e.Jmp(atDone)
		e.Mark(inPath)
		e.LoadStack(RSI, shaOffIn)
		e.LoadScaled8(RDX, RSI, RAX, 1, 0)
		e.Mark(atDone)
	}

	// --- block loop ---
	e.XorRegReg(RAX)
	e.StoreStack(RAX, shaOffPos)
	blk := b.fresh("sha$block")
	blkDone := b.fresh("sha$block$done")
	e.Mark(blk)
	e.LoadStack(RAX, shaOffPos)
	e.LoadStack(RCX, shaOffTotal)
	e.CmpRegReg(RAX, RCX)
	e.Jae(blkDone)

	// --- W[0..15] ---
	e.XorRegReg(R8) // t
	wT := b.fresh("sha$wt")
	wTDone := b.fresh("sha$wt$done")
	e.Mark(wT)
	e.MovRegImm32(RCX, 16)
	e.CmpRegReg(R8, RCX)
	e.Jae(wTDone)
	e.XorRegReg(R9)  // word accumulator
	e.XorRegReg(R10) // byte index i
	wI := b.fresh("sha$wi")
	wIDone := b.fresh("sha$wi$done")
	e.Mark(wI)
	e.MovRegImm32(RCX, 4)
	e.CmpRegReg(R10, RCX)
	e.Jae(wIDone)
	e.MovRegReg(RDX, R8)
	e.ShlRegImm(RDX, 2)
	e.AddRegReg(RDX, R10) // 4t + i
	e.LoadStack(RAX, shaOffPos)
	e.AddRegReg(RAX, RDX) // src = pos + 4t + i
	byteAt()
	e.ShlRegImm(R9, 8)
	e.OrRegReg(R9, RDX)
	e.IncReg(R10)
	e.Jmp(wI)
	e.Mark(wIDone)
	mask32(R9)
	e.StoreScaled64(R9, RSP, R8, 8, shaOffW) // W[t]
	e.IncReg(R8)
	e.Jmp(wT)
	e.Mark(wTDone)
	// --- W[16..63] ---
	// t lives in R11 and is never overwritten, so the store index survives
	// the four operand loads. Addition is commutative, so the operands are
	// ordered to keep R9 (the accumulator) free while s1 is computed.
	e.MovRegImm32(R11, 16)
	wX := b.fresh("sha$wx")
	wXDone := b.fresh("sha$wx$done")
	e.Mark(wX)
	e.MovRegImm32(RCX, 64)
	e.CmpRegReg(R11, RCX)
	e.Jae(wXDone)

	e.MovRegReg(RDX, R11)
	e.SubRegImm32(RDX, 15)
	e.LoadScaled64(R8, RSP, RDX, 8, shaOffW) // W[t-15]
	s0of()                                   // R9 = s0
	e.MovRegReg(RDX, R11)
	e.SubRegImm32(RDX, 7)
	e.LoadScaled64(RAX, RSP, RDX, 8, shaOffW)
	e.AddRegReg(R9, RAX) // + W[t-7]
	e.MovRegReg(RDX, R11)
	e.SubRegImm32(RDX, 16)
	e.LoadScaled64(RAX, RSP, RDX, 8, shaOffW)
	e.AddRegReg(R9, RAX) // + W[t-16]
	e.MovRegReg(RDX, R11)
	e.SubRegImm32(RDX, 2)
	e.LoadScaled64(R8, RSP, RDX, 8, shaOffW) // W[t-2]
	s1of()                                   // R10 = s1
	e.AddRegReg(R9, R10)

	// Addition wrapped at 2^64; the low 32 bits are already correct, so this
	// mask only clears the carry out of bit 31.
	mask32(R9)
	e.StoreScaled64(R9, RSP, R11, 8, shaOffW)
	e.IncReg(R11)
	e.Jmp(wX)
	e.Mark(wXDone)

	// ------------------------------------------------------------------
	// Stage 3: the compression rounds.
	//
	//	t1 = h + S1(e) + Ch(e,f,g) + K[t] + W[t]
	//	t2 = S0(a) + Maj(a,b,c)
	//	h=g; g=f; f=e; e=d+t1; d=c; c=b; b=a; a=t1+t2
	//
	// S1(e) = ROTR6 ^ ROTR11 ^ ROTR25, S0(a) = ROTR2 ^ ROTR13 ^ ROTR22.
	// Ch(e,f,g) = (e&f) ^ (~e&g) -- the only place NotReg is needed.
	// Maj(a,b,c) = (a&b) ^ (a&c) ^ (b&c).
	//
	// a..h live in the frame at shaOffV+0..+56 rather than in registers: there
	// are eight of them plus t1, t2, the round counter and the K pointer, and
	// the shift at the end of each round reads all eight at once. Keeping them
	// in memory is what makes the 64-round body a loop rather than 64 copies.
	// ------------------------------------------------------------------
	loadV := func(dst Reg, i int) { e.LoadStack(dst, shaOffV+i*8) }
	storeV := func(src Reg, i int) { e.StoreStack(src, shaOffV+i*8) }

	// ep1of: R8 holds x, result lands in R9. ROTR6/11/25 as ROL26/21/7.
	ep1of := func() {
		rol32(R8, RAX, RSI, 26) // ROTR6
		rol32(R8, RDX, RSI, 21) // ROTR11
		rol32(R8, RDI, RSI, 7)  // ROTR25
		e.MovRegReg(R9, RAX)
		e.XorRegReg2(R9, RDX)
		e.XorRegReg2(R9, RDI)
	}
	// ep0of: R8 holds x, result lands in R9. ROTR2/13/22 as ROL30/19/10.
	ep0of := func() {
		rol32(R8, RAX, RSI, 30) // ROTR2
		rol32(R8, RDX, RSI, 19) // ROTR13
		rol32(R8, RDI, RSI, 10) // ROTR22
		e.MovRegReg(R9, RAX)
		e.XorRegReg2(R9, RDX)
		e.XorRegReg2(R9, RDI)
	}
	// chOf: Ch(e,f,g) with e in R8, f in R10, g in R11 -> result in RDI.
	// (e&f) is formed BEFORE the complement, because NotReg destroys R8.
	chOf := func() {
		e.MovRegReg(RDI, R8)
		e.AndRegReg(RDI, R10) // e & f
		e.NotReg(R8)          // R8 = ~e
		e.AndRegReg(R8, R11)  // (~e) & g
		e.XorRegReg2(RDI, R8)
	}
	// majOf: Maj(a,b,c) with a in R8, b in R10, c in R11 -> result in RDI.
	majOf := func() {
		e.MovRegReg(RDI, R8)
		e.AndRegReg(RDI, R10) // a & b
		e.MovRegReg(RAX, R8)
		e.AndRegReg(RAX, R11) // a & c
		e.XorRegReg2(RDI, RAX)
		e.MovRegReg(RAX, R10)
		e.AndRegReg(RAX, R11) // b & c
		e.XorRegReg2(RDI, RAX)
	}

	// a..h = H[0..7] at the start of every block.
	//
	// This MUST read H, not V: V is the working state and holds whatever the
	// previous block left there (uninitialised stack on the first block). The
	// first draft of this helper wrote loadV/storeV, i.e. V[i] into V[i] -- a
	// self-copy that left `a` starting from stack garbage instead of the IV.
	// The symptom was precise and diagnosable: after one round only H[0] and
	// H[4] differed from the IV, because those were the only two slots the
	// round itself wrote (a = t1+t2 and e = d+t1).
	for i := 0; i < 8; i++ {
		e.LoadStack(RAX, shaOffH+i*8)
		storeV(RAX, i)
	}

	// --- 64 rounds ---
	e.XorRegReg(RAX)
	e.StoreStack(RAX, shaOffT)
	rnd := b.fresh("sha$round")
	rndDone := b.fresh("sha$round$done")
	e.Mark(rnd)
	e.LoadStack(RAX, shaOffT)
	e.MovRegImm32(RCX, 64)
	e.CmpRegReg(RAX, RCX)
	e.Jae(rndDone)

	// t1 = h
	loadV(RAX, 7)
	e.StoreStack(RAX, shaOffT1)
	// t1 += S1(e)
	loadV(R8, 4)
	ep1of() // R9
	e.LoadStack(RDX, shaOffT1)
	e.AddRegReg(RDX, R9)
	mask32(RDX)
	e.StoreStack(RDX, shaOffT1)
	// t1 += Ch(e,f,g)
	loadV(R8, 4)
	loadV(R10, 5)
	loadV(R11, 6)
	chOf() // RDI
	e.LoadStack(RDX, shaOffT1)
	e.AddRegReg(RDX, RDI)
	mask32(RDX)
	e.StoreStack(RDX, shaOffT1)
	// t1 += K[t]
	e.LoadStack(RAX, shaOffK)
	e.LoadStack(RCX, shaOffT)
	e.LoadScaled32(RDX, RAX, RCX, 4, 0)
	e.LoadStack(RAX, shaOffT1)
	e.AddRegReg(RAX, RDX)
	mask32(RAX)
	e.StoreStack(RAX, shaOffT1)
	// t1 += W[t]
	e.LoadStack(RAX, shaOffT)
	e.LoadScaled64(RDX, RSP, RAX, 8, shaOffW)
	e.LoadStack(RAX, shaOffT1)
	e.AddRegReg(RAX, RDX)
	mask32(RAX)
	e.StoreStack(RAX, shaOffT1)

	// t2 = S0(a) + Maj(a,b,c)
	//
	// t2 starts at ZERO. The first draft seeded it with `a`, which made t2 =
	// a + S0(a) + Maj(a,b,c) -- an extra +a that is exactly IV[0] on round 0,
	// so every digest came out rotated by that constant. It is invisible in the
	// emitted bytes (there is simply no store) and it leaves H[1..7] correct,
	// which is what made it look like an H-encoding fault.
	e.XorRegReg(RAX)
	e.StoreStack(RAX, shaOffT2)
	loadV(R8, 0)
	ep0of() // R9
	e.LoadStack(RDX, shaOffT2)
	e.AddRegReg(RDX, R9)
	mask32(RDX)
	e.StoreStack(RDX, shaOffT2)
	loadV(R8, 0)
	loadV(R10, 1)
	loadV(R11, 2)
	majOf() // RDI
	e.LoadStack(RDX, shaOffT2)
	e.AddRegReg(RDX, RDI)
	mask32(RDX)
	e.StoreStack(RDX, shaOffT2)

	// The eight assignments run in REVERSE dependency order so no value is
	// overwritten before it is read: h=g first (g is read before g itself is
	// replaced by f), and a = t1 + t2 last.
	loadV(RAX, 6) // g -> h
	storeV(RAX, 7)
	loadV(RAX, 5) // f -> g
	storeV(RAX, 6)
	loadV(RAX, 4) // e -> f
	storeV(RAX, 5)
	// e = d + t1
	e.LoadStack(RAX, shaOffT1)
	loadV(RDX, 3) // d
	e.AddRegReg(RDX, RAX)
	mask32(RDX)
	storeV(RDX, 4)
	loadV(RAX, 2) // c -> d
	storeV(RAX, 3)
	loadV(RAX, 1) // b -> c
	storeV(RAX, 2)
	loadV(RAX, 0) // a -> b
	storeV(RAX, 1)
	// a = t1 + t2
	e.LoadStack(RAX, shaOffT1)
	e.LoadStack(RDX, shaOffT2)
	e.AddRegReg(RAX, RDX)
	mask32(RAX)
	storeV(RAX, 0)

	e.LoadStack(RAX, shaOffT)
	e.IncReg(RAX)
	e.StoreStack(RAX, shaOffT)
	e.Jmp(rnd)
	e.Mark(rndDone)

	// H[i] += a..h[i]
	for i := 0; i < 8; i++ {
		loadV(RAX, i)
		e.LoadStack(RDX, shaOffH+i*8)
		e.AddRegReg(RDX, RAX)
		mask32(RDX)
		e.StoreStack(RDX, shaOffH+i*8)
	}

	e.LoadStack(RAX, shaOffPos)
	e.AddRegImm32(RAX, 64)
	e.StoreStack(RAX, shaOffPos)
	e.Jmp(blk)
	e.Mark(blkDone)
	// ------------------------------------------------------------------
	// Stage 4: the 64 lowercase hex digits.
	//
	// The C reference walks each 32-bit word from its top nibble down:
	//
	//	for (k = 7; k >= 0; k--) out[i*8 + (7-k)] = HEXD[(w >> 4*k) & 15];
	//
	// so word i occupies out[8i .. 8i+7] most-significant nibble first. The
	// shifts must be immediates, so this is unrolled 64 times; the alternative
	// would be a shift-by-CL primitive, which §11.1.1 avoids.
	// ------------------------------------------------------------------
	e.XorRegReg(RAX)
	e.StoreStack(RAX, shaOffO)
	for i := 0; i < 8; i++ {
		e.LoadStack(R8, shaOffH+i*8) // w
		for k := 0; k < 8; k++ {
			// nib = (w >> 4*(7-k)) & 15
			e.MovRegReg(RAX, R8)
			if s := byte(4 * (7 - k)); s > 0 {
				e.ShrRegImm(RAX, s)
			}
			e.MovRegImm32(RCX, 0xF)
			e.AndRegReg(RAX, RCX)
			// out[o] = tab[nib]
			e.LoadStack(RDX, shaOffTab)
			e.LoadScaled8(RAX, RDX, RAX, 1, 0)
			e.LoadStack(RDX, shaOffOut)
			e.LoadStack(RCX, shaOffO)
			e.StoreScaled8(RAX, RDX, RCX, 1, 0)
			e.LoadStack(RAX, shaOffO)
			e.IncReg(RAX)
			e.StoreStack(RAX, shaOffO)
		}
	}
	// out[64] = 0 -- the reference NUL-terminates, and the caller is handed a
	// length of 64, so the terminator is never part of the value.
	e.LoadStack(RAX, shaOffOut)
	e.XorRegReg(RDX)
	e.StoreMem8Off(RDX, RAX, 64)

	// Result: (RAX = out, RDX = 64). The caller moves the pair into (RDI, RSI).
	e.LoadStack(RAX, shaOffOut)
	e.MovRegImm32(RDX, 64)
	e.AddRsp(shaFrameBytes)
	e.Ret()
}
