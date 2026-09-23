# tinygo-riscv64-uefi

Adds the missing TinyGo runtime support needed to compile a `goarch=riscv64`
baremetal UEFI binary — the prerequisite for ever shipping `BOOTRISCV64.EFI`
out of the [`go-coff/stub`](https://github.com/go-coff/stub) project.

This project does **not** build TinyGo from source. It ships one Go file
that, once installed into TinyGo's runtime, lets `tinygo build` succeed for
the [uefi-riscv64.json](https://github.com/go-coff/stub/blob/main/targets/uefi-riscv64.json)
target. The intent is that this file lands upstream in TinyGo — see
[Upstreaming](#upstreaming) below.

## Status

| Stage                                 | State                                                            |
| ------------------------------------- | ---------------------------------------------------------------- |
| TinyGo runtime compile (Go side)      | ✅ resolved by `runtime/arch_riscv64.go` in this project          |
| go-coff stub `tinygo build` succeeds  | ✅ verified via `task verify`                                     |
| `BOOTRISCV64.EFI` link from COFF      | ✅ via `go-coff/peln` (`0x5064`); `lld-link` bypassed             |
| QEMU boot test                        | ☐ pending stub Taskfile wire-up (`link-riscv64` target)          |

> **Note** — earlier revisions of this table said `lld-link → BOOTRISCV64.EFI`
> was the blocker because LLD's COFF driver has no `/machine:riscv64`. That
> is true of `lld-link`, but the cloud-boot toolchain doesn't use it: the
> [`go-coff/peln`](https://github.com/go-coff/peln) library is a pure-Go COFF/PE
> linker built precisely to make a riscv64 UEFI build possible. It supports
> amd64 (`0x8664`), arm64 (`0xaa64`), and riscv64 (`0x5064`), with the
> RISC-V relocation table (`R_RISCV_HI20`, `_LO12_I`, `_LO12_S`,
> `_BRANCH`, `_JAL`, `_PCREL_HI20`/`_LO12_I`, `_32`, `_64`, `_RELAX`) unit-
> tested in [peln/linker/reloc_rv64_test.go](https://github.com/go-coff/peln/blob/main/linker/reloc_rv64_test.go).
> Invoke it via `pectl link --machine riscv64 …`.

## The TinyGo gap, fully explained

TinyGo's `src/runtime/` carries one `arch_<GOARCH>.go` per supported standard
Go architecture (`arch_amd64.go`, `arch_arm64.go`, `arch_386.go`, …). The
file declares per-arch constants (`GOARCH`, `TargetBits`, `callInstSize`,
`deferExtraRegs`), Linux ABI numbers (`linux_MAP_ANONYMOUS`, `linux_SIG*`),
and two helpers (`align`, `getCurrentStackPointer`). The rest of the runtime
(`gc_leaking.go`, `os_linux.go`, `panic.go`, `print.go`, `runtime_unix.go`)
uses these symbols unconditionally; without the per-arch file they show up
as `undefined`.

Counter-intuitively TinyGo *does* ship `arch_tinygoriscv64.go` already, but
that file is gated on the **TinyGo-internal** build tag `tinygo.riscv64` and
even pretends `GOARCH = "arm"` — it exists for the `-target=riscv-qemu`
embedded flow, not for the standard `GOARCH=riscv64` Go toolchain target.
Our UEFI target spec uses the *real* `goarch=riscv64`, so it picks neither
`arch_tinygoriscv64.go` (tag mismatch) nor any other `arch_*.go`.

[`runtime/arch_riscv64.go`](runtime/arch_riscv64.go) fills that gap. It is a
mechanical port of `arch_arm64.go`, with values adjusted for the RV64 LP64
ABI (same `TargetBits=64`, same 16-byte alignment, same `callInstSize=4`,
same signal numbers — RV64 Linux uses asm-generic for these).

## Try it

```sh
task verify
```

Output:

```
…
2026/05/17 17:14 → installed at /Users/…/Library/Caches/tinygo/goroot-…/src/runtime/arch_riscv64.go
2026/05/17 17:14 cd ../../go-coff/stub && rm -f main-riscv64.o && tinygo build -target=targets/uefi-riscv64.json -o main-riscv64.o .
-rw-r--r--  1 … staff   181K May 17 17:14 ../../go-coff/stub/main-riscv64.o
TinyGo compile OK. Note: lld-link CANNOT link this object into BOOTRISCV64.EFI yet …
2026/05/17 17:14 → removed /Users/…/src/runtime/arch_riscv64.go
```

After running, the TinyGo cache is back to its pristine state. Individual
sub-tasks:

| Task                  | Effect                                                       |
| --------------------- | ------------------------------------------------------------ |
| `task patch:apply`    | install the .go file into TinyGo's cached goroot             |
| `task patch:restore`  | remove it again                                              |
| `task patch:diff`     | print a unified diff ready to paste into a TinyGo PR         |
| `task verify`         | apply → compile go-coff stub → restore (the smoke test)      |

## Upstreaming

The proposed change is a single new file: `src/runtime/arch_riscv64.go`.
Suggested PR description:

> Add `arch_riscv64.go` so that `GOARCH=riscv64` can be used as the standard
> Go architecture (independent of the embedded `tinygo.riscv64` build tag).
> Mirrors `arch_arm64.go` with RV64-specific values: `TargetBits=64`,
> 16-byte alignment, `callInstSize=4`, signal numbers from
> `asm-generic/signal.h`.
>
> Motivating use case: UEFI baremetal stubs for `riscv64` (firmware uses the
> PE32+ format, machine `IMAGE_FILE_MACHINE_RISCV64`). With this file the
> runtime + `os_linux.go` typecheck successfully under `gc=leaking`,
> `scheduler=none`, `libc=""`.

`task patch:diff` emits the diff body.

## The remaining blocker: lld-link COFF/RISC-V

Once TinyGo emits `main-riscv64.o` (a COFF/PE-formatted relocatable
produced via the `riscv64-pc-windows-gnu` LLVM triple), the next step in
the pipeline is `lld-link` (the LLVM COFF/PE driver) wrapping it in a
PE32+ EFI application. As of LLVM 22.1.5, `lld-link` rejects
`/machine:riscv64`:

```
lld-link: error: unknown /machine argument: riscv64
```

LLD's COFF driver currently knows about AMD64, ARM, ARM64, ARM64EC,
ARM64X, I386, EBC. RISC-V PE/COFF support exists in the spec
(`IMAGE_FILE_MACHINE_RISCV64 = 0x5064` since UEFI 2.10 + PE32+ ECR) but
is not yet implemented in `lld/COFF/`.

This is a **separate** upstream gap; it lives in `llvm-project/lld/COFF/`,
not in TinyGo. Workarounds, in increasing order of effort:

1. **Wait** — track LLD's progress on RISC-V COFF/PE support.
2. **Bring-your-own stub** — point `cloud-boot build --stub=…` at a
   working `BOOTRISCV64.EFI` from a different project (e.g. a copy of
   systemd-stub, which ships riscv64 via different toolchain plumbing).
3. **ELF → PE32+ post-link wrapper** — produce an ELF binary with `ld.lld`,
   then wrap it in a PE32+ envelope by hand. The
   [`go-coff/peln`](https://github.com/go-coff/peln) library already knows how to mutate
   PE files; extending it to *construct* one from a raw image is a
   tractable next step.

## Layout

```
tinygo-riscv64-uefi/
├── README.md                # this file
├── Taskfile.yaml            # patch:apply / patch:restore / patch:diff / verify
├── runtime/
│   └── arch_riscv64.go      # the proposed TinyGo file
└── testdata/                # reserved for future smoke fixtures
```
