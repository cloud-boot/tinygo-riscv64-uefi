// This file is the proposed addition to TinyGo's
// src/runtime/arch_riscv64.go. It mirrors arch_arm64.go (same intent,
// same fields, riscv64-specific values).
//
// The build constraint matches the *standard* Go architecture name
// `riscv64`, not the TinyGo-specific `tinygo.riscv64` (which has its
// own runtime under arch_tinygoriscv64.go and is intended for the
// embedded -target=riscv-qemu path with GOARCH="arm").
//
// Once TinyGo has this file, our UEFI target spec
// targets/uefi-riscv64.json (goos=linux, goarch=riscv64, gc=leaking,
// scheduler=none, libc="") will compile and produce a PE/COFF object
// suitable for lld-link to link into BOOTRISCV64.EFI.

//go:build riscv64

package runtime

const GOARCH = "riscv64"

// TargetBits is the address width of the CPU. 64 for riscv64 (RV64),
// matching arm64 / amd64.
const TargetBits = 64

// deferExtraRegs is the number of extra callee-saved registers a defer
// frame needs to spill on this arch. arm64 saves 0 (the existing
// callee-saved set is already captured by the runtime), and the RV64
// LP64 ABI exposes the same callee-saved register class (s0..s11) that
// TinyGo's defer mechanism doesn't need to spill separately.
const deferExtraRegs = 0

// callInstSize is the size in bytes of a single call instruction. The
// runtime walks back from the return address by this many bytes to find
// the start of the call. RISC-V uses `jal` (4 bytes) for direct calls
// within ±1 MiB; far calls become `auipc + jalr` (8 bytes) but the
// return address still points immediately after `jalr`, so the
// conservative value of 4 matches how the existing tinygoriscv runtime
// declares it (arch_tinygoriscv.go).
const callInstSize = 4

// Linux signal / mmap constants. RISC-V Linux uses the *generic* values
// (asm-generic/mman.h, asm-generic/signal.h) — same as arm64 / x86_64
// for these particular constants.
const (
	linux_MAP_ANONYMOUS = 0x20
	linux_SIGBUS        = 7
	linux_SIGILL        = 4
	linux_SIGSEGV       = 11
)

// align rounds ptr up to the next 16-byte boundary. RV64 LP64 ABI
// (https://riscv.org/wp-content/uploads/2015/01/riscv-calling.pdf §16)
// mandates 16-byte stack alignment at function entry, same as arm64.
func align(ptr uintptr) uintptr {
	return (ptr + 15) &^ 15
}

// getCurrentStackPointer returns the current SP register value. Uses
// LLVM's @llvm.stacksave intrinsic via TinyGo's stacksave() shim —
// arch-agnostic at the IR level, so the riscv64 backend lowers it to a
// trivial `mv result, sp`.
func getCurrentStackPointer() uintptr {
	return uintptr(stacksave())
}
