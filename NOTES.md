# Robinson's Requiem — Go port notes

## Sources

- `orig/cd/` — the CD files (extracted from `orig/robin.iso`), `START.STP` from the install.
- `orig/start32.bin` — START.EXE's 32-bit flat image (X-32 extender; file offset 0x4857 = address 0,
  entry EIP 0xd, code 0..0x14d80, data 0x14d80..0x1cba8, BSS to 0x1cd50). Ghidra project in `ghidra/rrq`.
- `ref/alis/` — maestun/alis (C reimplementation of the ALIS VM, MIT) plus a deterministic harness
  (`src/harness.c`): virtual time, scripted input, frame CRC log, PNG dumps.

## Machine

ALIS 3.1, PC, little endian, 320x200x8, 4 MB arena, basemem 0x2b400, context 0x3e bytes,
double buffered. The main script header carries the VM specs.

Timing model (same in Go and the C harness): virtual µs clock that only advances in VM sleeps;
timer ISR (`itroutine`) every 17 ms, audio mix (882 stereo frames) every 20 ms, display sample
every 20 ms.

## alis behaviour still to check against START.EXE

- `trinorme` (sort for near/view with callentity): reads buffer[-1], can loop forever.
- `traitmat` / `multimat` / `clipmat`: Atari-derived, s16 truncation of form addresses.
- `cescape`: truncates `calscal` to s16.
- `csetmouse`: y comes from a stale D6.
- `cworld`: alis writes vram+0xffde/0xffdf; the port writes ctx -0x22/-0x21.
- `cdefworld`: alis writes all 6 bytes to the same address.
- Handlers missing in alis for v3.1 (consume no operands): cinitab, cfcreat, cfdel,
  cfwritei, cpicture, cfindmat, cnearcla, cviewcla, csczoom, cautomode,
  cscaset, cscamov, cscsun, ccancel, and the oper oright.
- `cfreadb`: returns before reading the length operand when the handle is empty.
- `walkland` head bob sampling; `landalti` flag==1 pointer add is a no-op.
- `cgettime`: alis returns 0,0,0; original returns the DOS time.
- addnames: alis sets the zero flag only for alocb/alocw/adirb/adirw (x86 `add` sets ZF always),
  `cloop` reads it.
- `cnul`: alis stops the VM; the original returns.
- `shrinkprog` resets `vacc_off` of shifted entities (only matters for the running one).
- `copenfilm`: alis maps "150." films to "300.".
- Sound: alis plays czap/cexplode/cding/cnoise on an emulated YM2149 (Atari); the DOS engine
  renders them for the Sound Blaster.
- `csend`: queue-full handling; `cmouse`: both buttons; `cfreadv`/`cfwritev`: big-endian words.
- `cbackstar`: mode byte taken from basemain+1; `ctexmap`: opaque-row scan and y_mask snap;
  `cdarkpal`: checks only the first 0xb6 entries; `csettime`/`cgettime`: host clock + offset.
- `affiscr`: alis holds `linkidx` (clinking) in a u16, so the `linkidx >= 0` / `< 0` sign tests
  are constant; the original tests the sign (unlinked sprites skip `suitlin1`).
- `calcfen`: alis aligns the window to 16 px after clamping; the DOS original aligns before.
- `dos_pal_tick`: fade steps at 50/58.8 of the 17 ms tick (alis pace); the original steps on VGA vblank.
- `memfen`: tabfen overflow is unchecked in alis (port clamps).

## Port status

Everything runs: VM, image pipeline, screens, the rrq3d landscape (DOS path), FLI films, sample
audio, the deterministic harness (cmd/rrqharness) and the native GLFW+wgpu+oto host (cmd/rrq).
`go test ./cmd/rrqharness` replays testdata/{attract,start,walk}.txt and requires the CRC log to
equal the C reference log (testdata/*.c.log) frame for frame.

The C reference converts 4-bit sprites to 8-bit in place at load (ALIS_CONV_INPLACE), so its
script addresses sit higher than the original's (and the Go port's); the rendered output is the same.

MV2 music (the sample player, internal/alis/music_v2.go) is ported and bit-exact (testdata/play6).
alis never fills `mv2a.trkval`, so transposed instruments read the struct fields after it; the
port copies that (`trkvalAt`).

State check: `rrqharness -memlog f -memevery n` and the C harness `ALIS_MEMLOG`/`ALIS_MEMLOG_EVERY`
log a CRC of the whole 4 MB arena; `-memdump`/`ALIS_MEMDUMP(_AT)` dump it. Compare against
`alis_noconv` (the reference built with ALIS_CONV_INPLACE=0, whose arena layout equals the port's).
All testdata sessions have identical arenas. The reference patches (harness, traces, fixes) are in
tools/alis-ref.patch, applied to maestun/alis.

Reference bugs found this way (fixed in the reference, the port follows START.EXE):
- barland hit test: alis sign-extends the cell type byte into `ntstpix`; START.EXE zero-extends it.
- Variable operands are signed: alis added `script_read16()` (u16) to vram_org/basemain in the
  store/add/oper names; START.EXE uses `movsx` (e.g. tabstring 0xa199, cloop 0x3aae).
- `cret`: alis compared the u16 `saversp` with the s16 `vacc_off`, so a top-level return never
  stopped the script; START.EXE 0x3a53 compares signed words (`jl`).
- him* (another entity's variables, opers 0x9ebf.., stores 0xa8cb.., adds 0xab86..): the byte and
  word forms check the entity index (error screen 0x1b/0x1d/0x1f/0x21, "him négatif") and skip
  dead entities (vram <= 0, D7 unchanged for reads); the operands of the table forms are
  (offset, entity variable). alis lacked ahimb, ahimp, ahimtc, ahimti and the table reads; the
  port lacked shimp..shimti. The reference has a v3.1 path in src/him31.h.
- Form centre (`getVector`, START.EXE 0x7176, cwmov/cwlive/cwtstmov): bytes are signed, the
  three values are x/y/z in order (alis swapped y and z), the form address is unchecked, the
  mirror test is a signed `>= 0x15`; a negative form type is error 20 ("vecteur multiple").
- ctstset, cwtstmov, cwftstmov were missing in alis (now ported from 0x74d4/0x7248/0x729a).
- The test opcodes latch the own mirror flag (0x1602e) for monoform; cavtstmov doesn't set it.

Differential tests (`go test -tags difftest ./internal/alis`, internal/alis/cref links
ref/alis/build/libalisref.a): arena after init and after 8 s of play, sample mixer, MV2 player,
both unpackers (all scripts and random streams), and TestOpcodeFuzz: from the same live state,
random instructions of the expression/flow opcodes in both VMs, comparing arena and registers.
START.EXE's dispatch tables: opcodes 0x18432 (dword per opcode), opernames 0x1884e, storenames
0x189a2, addnames 0x18a1a (dword per even code, `jmp [eax*2+table]`); `r2 -a x86 -b 32 -m 0
orig/start32.bin` disassembles them.

The harnesses keep files the game writes (CONFIG.INF) in `-save` / `ALIS_SAVEDIR`, so runs
don't leak state into each other through the data directory.

Audio still missing (never seen in play):
- the mv1 player and the mv2 chip/OPL2 players (RRQ's songs use samples);
- the OPL2 effect channels (DingZap/Noise/Explode, `runson`): alis renders them with emu8950
  using register settings taken from Ishar 2; ioCanal is a no-op.
- alis sys_init runs before the platform is known, so sys_timeclock_hz is 50
  although the ISR ticks every 17 ms: palette fades step on every tick and
  waitclock films wait waitclock*50/60 ticks. The Go port copies this
  (timeclockHz); check START.EXE.
