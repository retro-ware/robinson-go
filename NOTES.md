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

- `walkland` head bob sampling; `landalti` flag==1 pointer add is a no-op.
- `cgettime`: alis returns 0,0,0; original returns the DOS time.
- `shrinkprog` resets `vacc_off` of shifted entities (only matters for the running one).
- `copenfilm`: alis maps "150." films to "300.".
- Sound: alis plays czap/cexplode/cding/cnoise on an emulated YM2149 (Atari); the DOS engine
  renders them for the Sound Blaster.
- `csend`: queue-full handling.
- `cbackstar`: mode byte taken from basemain+1; `ctexmap`: opaque-row scan and y_mask snap;
 `csettime`/`cgettime`: host clock + offset.
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
- Also ported from START.EXE (missing in alis for v3.1): cinitab, cfcreat, cfdel, cfwritei,
  cpicture, cautomode, ccancel, oright; the error screen texts (0x1c626) are `alisErrors`.
- near/view/find (0x7ac6..0x842e, reference in src/near31.h): 16-bit coordinate differences,
  signed 32-bit distance compares, the norm squared as u16 (`mul dx`), the cone factor
  zero-extended; the material matcher compares the low 16 bits of the addresses, a sub-form
  header tests the whole word's sign; the searches write only the low byte of matmask;
  cfindtyp/cneartyp compare the raw type word. alis' trinorme never ended once it swapped.
  cfindmat loads goodmat from matent (original bug, kept). cnearcla/cviewcla take a word
  operand; START.EXE's nearest-only path there swaps bx/dx as 16-bit registers and corrupts
  its pointers, so the port follows the other searches.
- Zero flag after an add destination (`cloop`): every byte/word add sets it from the result,
  the string forms leave it set, the stack push clears it, a dead ahimb/ahimw sets it to
  vram == 0 (alis set it only for alocb/alocw/adirb/adirw).
- `cdefworld` copies 5 bytes in a row (alis: 6 bytes to one address); `cworld` writes the word
  at vram-0x22 (alis: vram+0xffde).
- `cescape` compares signed 32-bit products (alis: s16); capproach/cescape test only the
  16-bit operand against -1; `csetmouse` takes y from its second operand (alis: stale D6).
- `cdarkpal`: unsigned first/count, darkness flag from all 0x2fd words after entry 0 (alis: 0xb6).
- `cfreadb`: a non-positive pointer cell is error 26 (0x9796); `cnul` (0x00, 0x37, 0x8b) returns.
- `cmouse` returns the driver button word (both buttons = 3; alis: 1); cfreadv/cfwritev are
  big-endian as in alis.
- Camera words of the current screen (missing in alis): cscamov/cscaset 0x34 (add/set),
  csczoom 0x3a, cscsun 0x7c (four words); the screen is looked up after the operands.
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

## Handover checklist (current work: play2/play6 sessions vs the START.EXE oracle)

Reproduce: `go build -tags orig -o X/origrun ./cmd/origrun`, run from the repo root
`X/origrun -ms 600000 -frames X/p.orig -input testdata/play2.txt`; the port from orig/cd:
`rrqharness -data . -maxms 600000 -frames X/p.go -input ../../testdata/play2.txt -save X/h`;
`cmp` the two frame logs.

- [x] play2 matches the oracle for 120 s (5462 draws).
- [x] First divergence: draw 12515, tick 14258 = `crunfilm` of socrate.dat (flags 0x203).
      Script state is identical, so it's rendering only: the port overwrites palette 160..255 and
      clears the HUD.
- [x] Read START.EXE's film code: open 0x8e5b (codesc1 6), run 0x13da8, del 0x13d53, inifilm
      0x13c2f, step 0x13f08, header 0x1412c, frame 0x14164, chunk dispatch 0x14198, palette
      0x14270, delta 0x14358, black 0x143bc, brun 0x14418, copy 0x14490, audio 0x14211, header
      clear 0x143d5. Film struct 0x17944: +0 flags, +4 frame, +6 frames, +0x44 result,
      +0x46 batchframes, +0x48 waitclock, +0x4a basemain, +0x4e file handle.
      Flags: bit0 = brun skips 0 pixels, black chunk skipped, palette goes through 0x8adf when
      0x178fb != 0 (set by 0x8871..); bit1 = a 256-colour palette packet sets only 160;
      bit2 = decode into logic 0x178db and copy it to physic after each frame (else decode into
      physic 0x178eb); bit3 = clear palette and screen at the header; bits 8..15 = audio
      channels 0x1000+n. Decoding is always 320 wide at offset 0 (no window, no film width);
      palette packet indices are absolute; only header 0xaf11 and frame 0xf1fa (others end).
- [x] Rewrote internal/alis/film.go's decoder after START.EXE (0x178fb = img.fdarkpal, set by
      cdarkpal 0x87e8; 0x8adf = dpalDarkInto, black→2,2,2 test now signed as in START.EXE).
      Removed the "150."→"300." film rename (START.EXE opens the name as given).
- [x] Removed the debug prints. Kept the oracle's film sound hooks (orig/hooks.go 0x109b2/0x109c7:
      idle while START.EXE busy-waits for the mixer).
- [x] intro90/start60/walk118 still pass; socrate.dat now draws pixel-identical frames.
- [x] Film pacing: internal/alis/sb.go models the SB block interrupt (every 0x400 samples at
      tc 190 = 80640 PIT clocks, first one block after the game timer starts) and the film's
      mixer channel; the push 0x10941 spins until the mixer takes the block. The "skip the frame
      wait" flag of 0x14269 lands in the mixer channel (edi was reloaded), so the film always
      waits for its tick (original bug, kept). Host audio still comes from the alis mixer;
      the other mixer channels aren't modelled (port 0x10b7e/0x10c71.. when sound must match).
- [x] play2 and play6 now match the oracle up to the map sprites fatal (draw 13281 / 17815).
- [x] Map sprites: internal/alis/gfx_map.go ports 0x12c16 (sprite words to 0x1c0fe), type 9
      textured maps 0x13168 and the texel 0x13411; type 10 draws nothing; tile maps
      (0x12c69..0x13163) call fatal and are still to port when a session reaches them.
- [x] Palette: dpalTopalette fades only when dx != 0 and mip (0x16064) > 200 (0x8988/0x21ed);
      dpalCtopalsav's fdarkpal black test is on the 6-bit values (0x8c8b).
- [x] cchartmap's clear (0x4f14, ops_chart.go chartClear) reads the script's own position
      es:[ebp]/es:[ebp+8], not the wcx/wcy work registers. This was the play6 map-screen
      mismatch at draw 45898.
- [x] play2 and play6 match the oracle for their whole 1260 s inputs (72035 draws each);
      intro90/start60/walk118 pass; gofmt and go vet are clean.
- [x] Golden tests: testdata/play2.orig.frames and play6.orig.frames; harness_test.go runs them
      only with RRQH_LONG=1 (about 23 s for all five). The default `go test ./...` takes about 3 s.
- [x] Tile maps ported (gfx_map.go blitTileMap 0x12c69, drawTile 0x12ea1) with the 16-bit
      `add si,8`/`add di,dx` quirks and cells read at 0x10000|low16. They are UNTESTED, and probably dead
      in this game: a bytecode scan finds cdefmap only in MAIN.IO, with types 10 and 9.
- [x] Sound rewrite: START.EXE's DOS sound system now lives in v.mem at its DS addresses.
      sb.go covers init 0x107cf/0x10c93/0x10d6b, the IRQ 0x10edb, mix 0x10ac1/0x10b7e,
      start/stop/alloc and the film channel/push. ops_snd.go covers csound, cmsound, cinstru,
      cmusic, cdelsound and csetvolum. The ops_io beeps (czap and so on) only store their
      operands, as on DOS. audio.go resamples the 15 kHz blocks for the host. The old alis
      mixer and music_v2.go were removed, along with their difftest cases.
- [x] Verification: run origrun/rrqharness with -sblog. For play2 and play6 (120 s), every
      block's end time and CRC matches the oracle once the oracle's times are measured from its
      game timer start (oracle t 9895213 = port t 0). The oracle logs 3 extra silent blocks
      first. Two of them come from the IRQ 5/7 probe (0x11097), which runs before the timer is
      installed. The third plays from t 0 and the port doesn't log it. The oracle's 8 s before
      that are the DOS loader stub. So audio timing is exact, and nothing needed fixing.
      Frame goldens still pass.
- [x] Module player ported (music.go, 0x1158e..0x123e7: start/play/fade/stop, tick, rows,
      effects, retrigger, funk). The AdLib path 0x1194a is dead (no type-6 instruments) and
      calls fatal. Verified with a test data dir whose LOGOSND.IO is MUSIQUES.IO (or
      MUSIQUE2.IO) stored unpacked with id 0x89, so MAIN plays it at once: MUSIQUES matches
      for all 3550 blocks (240 s), MUSIQUE2 for 448 blocks until the oracle crashes there
      (flags.IO overwrites the samples in that setup, a test artefact). Stored format:
      le32 len+6, bytes 01 00, data.
- [x] convertSamples follows 0x2bf3: only packed type 1/2 samples with byte 6 == 1, the
      packed bytes copied to [0x15ee8] and Fibonacci-decoded in place, the last two bytes
      zeroed and the length cut by 2.
- [x] Films: chunk 0x10 copies 64000 bytes whatever its size (0x14490); the palette waits
      for the retrace (0x1433b calls 0x1371d, clock.go waitRetrace on the VGA dot clock).
- [x] Film streaming (film.go): copenfilm's films stream through a VM-memory buffer
      (inifilm 0x13c2f, open/step 0x13f08, refill 0x13fd0, header 0x1412c, del 0x13d53),
      reading cdspeed*waitclock/0x4da3 - 1/8 bytes per frame; a frame not read in time ends
      the film (intro24d.dat ends at frame 138 of 301 as in the oracle). The CD speed
      [0x18abc] is 9000000 as the oracle measures it (core.go). cfilm leaks the handle and
      buffer as 0x8e13 does.
- [x] Intro session testdata/intro.txt (UK flag clicked at 60 s, mouse 110 140; FLI intro
      after it). Frames match the oracle for 600 s (golden testdata/intro.orig.frames, long
      test) and every SB block matches. origrun has -cd DIR for alternative data dirs.
- [x] omod/odiv follow 0xa305/0xa31f: a 16-bit idiv with a zero divisor taken as 1 (D6
      gets the divisor). Only -32768 / -1, which traps on x86, still calls fatal.
- [x] operAX (cscfollow's last operand, gfx_scene.go) covers every operand form; oeval
      (0x38) always leaves 0x3a, the code of the ofin (0xa767) that ends its loop.
      Table forms take ax from tabint/tabstring's last term (tabAX, 0xa0f3/0xa199).
- [x] Divide errors: START.EXE hooks interrupt 0 (0x2045 installs 0x20d9). The handler
      shifts the dividend right by 2 and re-runs the division: all of edx:eax (logical) when
      ModRM bit 2 is clear (idiv bx/cx/ebx/ecx, div bx/cx), only dx:ax (arithmetic) when it's
      set (memory operands, si, di, bp). A zero divisor retries for ever (the original hangs).
      The port's model is xdiv/DivRetry (gfx_land_math.go), used by every land division,
      draw's idiv16 (0x5990) and cdefmap. 16-bit sites in mode A also shift in the upper
      halves of eax/edx, so landPersp/landPersp2/landFA29/0xd527/0xee9c take them from their
      callers. A zero divisor hangs as in the original (divHang: only the interrupts run on,
      also at 0xcc78, 0x4bab, 0x14254 and 0xf2ac). Left as fatal, because their retries shift
      in edx's upper half from before the opcode, which the port doesn't model (D7 is kept
      sign-extended): omod/odiv's -32768 / -1, 0xf2ac with sky width 0xfffe and ax -32768,
      and a speech rate over 0xffff at 0x14254.
      This was the "land: division fault" a play session hit.
- [x] Oracle: unicorn never reset QEMU's env->old_exception when the hook took an exception,
      so the second divide error became a double fault and the third a triple fault (CPU
      halted). oldexc.go finds the field by probing and resets it after each exception.
      internal/orig/div_test.go checks DivRetry against the real handler bytes (10 cases,
      up to 16 retries each).
- [ ] Nothing is committed yet (branch port-wip).
- [x] Crash reports: `alis.Error.Report()` (internal/alis/crash.go) gives the message, script/pc/entity/time, the last 64 opcodes and the Go stack, for `fatal` and for Go panics recovered in `Run` (which now returns the error instead of re-panicking). cmd/rrq prints it and writes `crash-YYYYMMDD-HHMMSS.txt` to the save dir; rrqharness prints it to stderr.
- [x] 3D freeze (division by zero in `caim` → `landAtan`): the port's accumulator stack started at 0x19c00, inside START.EXE's sine-table mirror (0x199be..0x19c8e), so pushed values and strings corrupted the tables. It now starts at 0x18328 like START.EXE's `ebx` (0x9d24); 0x18042..0x18328 is free. Divide-by-zero hangs also write a crash report (`VM.OnHang`).
- [x] Accumulator cells are words as in START.EXE (`push` keeps the low word, `pop` sign-extends; table indices are popped with `movsx`, 0xa0a8); `oprnd` rotates the popped word (`ror ax, 6`, 0xa3ec).
