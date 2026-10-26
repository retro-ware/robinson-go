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
- Handlers missing in alis for v3.1 (consume no operands): ctstset, cinitab, cfcreat, cfdel,
  cfwritei, cpicture, cfindmat, cnearcla, cviewcla, cwtstmov, cwftstmov, csczoom, cautomode,
  cscaset, cscamov, cscsun, ccancel.
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

Opcode handlers are all ported (internal/alis/ops_*.go, table in ops_table.go). The package does
not compile yet: missing are the VM struct rewrite (vm.go), image.c pipeline (sprite lists,
put/putin/putmapin, affiscr/fenetre/destofen, blitters, requiem map, DOS palette, itroutine),
screen.c (scadd/scbreak/scdosprite/vectoriel), rrq3d (openland/affiland/landtopix/landtofi/
pointpix), FLI films, audio (channels, playsample/runson, mv2 tracker, mixer), the virtual-clock
scheduler/harness and the GLFW+wgpu host.
