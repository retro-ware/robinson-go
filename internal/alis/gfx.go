package alis

// The image code of START.EXE keeps its state in its data segment. The
// port keeps those globals in v.mem at the original DS offsets, and lays
// out the arena, the screens and the sprite segment at the original
// addresses, so that its memory compares byte for byte with the original's.

const (
	memTop  = 0x8ad200 // end of the X-32 block: the first top of memory (0x15f26)
	vgaBase = 0xa0000  // video memory
	vgaSize = 320 * 200

	// X-32 block, carved by the setup at 0x1b97 and 0x2cf6
	memBlock  = 0x10c000 // 0x15ee8, 0x15f1e: 0x8100 bytes
	memBlockB = 0x114100 // 0x15ef0: 0x81a8 bytes
	logicBase = 0x11c2a8 // logic screen (0x178db), 0xfa00 bytes
	progBase  = 0x12bca8 // first program (0x15f22 after the logic screen)

	tabptr = 0x18adc // 16 memory handles of 8 bytes (memalloc 0x1c58)
)

// sprite segment
const (
	dsDebsprit = 0x15f84 // first byte of the sprite segment
	dsFinsprit = 0x15f88 // end of the sprite segment
	dsFsBase   = 0x15f8c // fs base (= debsprit)
	dsLibsprit = 0x15f90 // free sprite list
	dsTvsprite = 0x15f92 // sprite 2: the whole screen
	dsSprite62 = 0x15f96
	dsMouseAd  = 0x15f98 // address of the mouse sprite
	dsSprite32 = 0x178b5
	dsMouseSpr = 0x16074 // mouse sprite
	dsMouseOn  = 0x16076

	sprSize = 0x30
)

// sprite record fields (fs:[di+X])
const (
	spState    = 0x00 // 0 drawn, 1 to erase, 2 changed, 0xff new, 0xfe hidden
	spNumelem  = 0x01
	spScreen   = 0x02 // scene id
	spToNext   = 0x04 // element list of the entity (and the free list)
	spLink     = 0x06 // draw list of the scene
	spNewad    = 0x08 // drawn: address of the resource table entry
	spNewx     = 0x0c
	spNewy     = 0x0e
	spNewd     = 0x10
	spData     = 0x12 // put: address of the resource table entry
	spDepx     = 0x16
	spDepy     = 0x18
	spDepz     = 0x1a
	spCredon   = 0x1c
	spCreduc   = 0x1d
	spClinking = 0x1e
	spCordspr  = 0x20
	spZoom     = 0x21
	spNewf     = 0x22 // drawn mirrored (bit 0)
	spInvx     = 0x23 // put mirrored
	spEntity   = 0x24 // entity (vram -0xe)
	spColour   = 0x26 // 0x80 at +0x27: drawn in the colour at +0x26
	spWidth    = 0x28
	spHeight   = 0x2a
	spZoomx    = 0x2c
	spZoomy    = 0x2e
)

// element globals
const (
	dsFerase   = 0x15f9d
	dsBasemain = 0x15fa0
	dsOldcx    = 0x15fa4 // savecoord: position and angles of the running entity
	dsOldcy    = 0x15fa6
	dsOldcz    = 0x15fa8
	dsOldacx   = 0x15faa
	dsOldacy   = 0x15fac
	dsOldacz   = 0x15fae
	dsDepx     = 0x15fb2 // put offset
	dsDepy     = 0x15fb4
	dsDepz     = 0x15fb6
	dsNumelem  = 0x15fbe // word
	dsInvx     = 0x15fc4
	dsPutSub   = 0x15fc5 // put of a group member
	dsPutAgain = 0x15fc6
	dsPtscreen = 0x15fc8
	dsMonoCol  = 0x15f42
	dsMonoOn   = 0x15f43
	dsFlagmain = 0x15fc7 // resources from the main script
)

// draw globals
const (
	dsT9a    = 0x15e9a // scratch words of the draw code
	dsT9c    = 0x15e9c
	dsT9e    = 0x15e9e
	dsTa0    = 0x15ea0
	dsTa2    = 0x15ea2
	dsTa4    = 0x15ea4
	dsTa6    = 0x15ea6
	dsTa8    = 0x15ea8
	dsLinkid = 0x15eaa
	dsScene  = 0x15eba // address of the scene being drawn
	dsTc2    = 0x15ec2
	dsTc4    = 0x15ec4
	dsTc6    = 0x15ec6
	dsTc8    = 0x15ec8
	dsDrawn2 = 0x15ee2

	dsFclip  = 0x15fca // the rectangle 0x15fd6 is set
	dsBlocx1 = 0x15fcc // rectangle to redraw
	dsBlocy1 = 0x15fce
	dsBlocx2 = 0x15fd0
	dsBlocy2 = 0x15fd2
	dsBlocw8 = 0x15fd4
	dsClipx1 = 0x15fd6
	dsClipy1 = 0x15fd8
	dsClipx2 = 0x15fda
	dsClipy2 = 0x15fdc
	dsFenOn  = 0x15fe8
	dsFenE9  = 0x15fe9
	dsFenx1  = 0x15fea // scene window within the rectangle
	dsFeny1  = 0x15fec
	dsFenx2  = 0x15fee
	dsFeny2  = 0x15ff0
	dsFenl   = 0x15ff2
	dsFenh   = 0x15ff4
	dsDrawn  = 0x15ff6
	dsMoveSc = 0x15ff7

	dsNewad    = 0x16014 // 0x587a: where an element is drawn
	dsNewx     = 0x16018
	dsNewy     = 0x1601a
	dsNewd     = 0x1601c
	dsNewl     = 0x1601e
	dsNewh     = 0x16020
	dsNewzoomy = 0x16022
	dsNewzoomx = 0x16024
	dsNewf     = 0x16026

	dsWback   = 0x178b2
	dsLandRd  = 0x1aa75
	dsSrcSkip = 0x1bfc0 // blitter row skips, colour and nibble phase
	dsDstSkip = 0x1bfc4
	dsBlitCol = 0x1bfc8
	dsNibble  = 0x1bfc9
	dsMapSpr  = 0x1c0fe // copy of the map sprite record
)

// screens
const (
	dsBacklog  = 0x178bb
	dsWlogic   = 0x178cb
	dsWlogx1   = 0x178cf
	dsWlogy1   = 0x178d1
	dsWlogx2   = 0x178d3
	dsWlogy2   = 0x178d5
	dsWloglarg = 0x178d7
	dsLogic    = 0x178db
	dsLogx1    = 0x178df
	dsLogy1    = 0x178e1
	dsLogx2    = 0x178e3
	dsLogy2    = 0x178e5
	dsLoglarg  = 0x178e7
	dsLoghaut  = 0x178e9
	dsPhysic   = 0x178eb
	dsScrW     = 0x178ef
	dsScrH     = 0x178f1
)

// mouse cursor
const (
	dsCurVis    = 0x1606a // the cursor is in the rectangle being copied
	dsCurX      = 0x1606c
	dsCurY      = 0x1606e
	dsCurSave   = 0x16070 // background under the cursor
	dsCurShown  = 0x16077 // < 0 hidden
	dsCurLock   = 0x16078
	dsCurSprite = 0x1607a
	dsCurW      = 0x18e3f
	dsCurH      = 0x18e41

	curSaveBuf = 0x1832c
)

// fsa is the address of fs:[di+off]: the sprite segment takes 16-bit
// offsets from debsprit.
func (v *VM) fsa(di, off uint16) uint32 { return v.debsprit + uint32(di+off) }

func (v *VM) fs8(di, off uint16) uint8          { return v.mem[v.fsa(di, off)] }
func (v *VM) fs16(di, off uint16) int16         { return v.r16(v.fsa(di, off)) }
func (v *VM) fsu16(di, off uint16) uint16       { return v.ru16(v.fsa(di, off)) }
func (v *VM) fsu32(di, off uint16) uint32       { return v.ru32(v.fsa(di, off)) }
func (v *VM) setFs8(di, off uint16, x uint8)    { v.mem[v.fsa(di, off)] = x }
func (v *VM) setFs16(di, off uint16, x int16)   { v.w16(v.fsa(di, off), x) }
func (v *VM) setFsu16(di, off, x uint16)        { v.wu16(v.fsa(di, off), x) }
func (v *VM) setFsu32(di, off uint16, x uint32) { v.wu32(v.fsa(di, off), x) }

// sca is the address of scene id (basemain + movsx id).
func (v *VM) sca(id uint16) uint32 { return v.basemain + uint32(int32(int16(id))) }

// vram context fields of an entity
func (v *VM) screenIDOf(vram uint32) uint16 { return v.ru16(vram - 0x16) }
func (v *VM) curScreenID() uint16           { return v.ru16(v.script.vramOrg - 0x16) }
func (v *VM) elemHead() uint16              { return v.ru16(v.script.vramOrg - 0x18) }
func (v *VM) xinv(vram uint32) uint8        { return v.mem[vram-0x03] }
func (v *VM) cforme(vram uint32) int16      { return v.r16(vram - 0x1a) }

// initScreens sets up the logic screen, the video memory and the windows
// (START.EXE 0x2cf6, 0x13692).
func (v *VM) initScreens() {
	v.wu32(dsPhysic, vgaBase)
	v.wu16(dsScrW, gameWidth)
	v.wu16(dsScrH, gameHeight)
	v.wu16(dsLoglarg, gameWidth)
	v.wu16(dsLoghaut, gameHeight)
	v.wu32(dsLogic, logicBase)
	v.wu16(dsLogx1, 0)
	v.wu16(dsLogy1, 0)
	v.wu16(dsBlocx1, 0)
	v.wu16(dsBlocy1, 0)
	v.wu16(dsLogx2, gameWidth-1)
	v.wu16(dsBlocx2, gameWidth-1)
	v.wu16(dsLogy2, gameHeight-1)
	v.wu16(dsBlocy2, gameHeight-1)
	v.wu32(dsWlogic, logicBase)
	v.wu16(dsWloglarg, gameWidth)
	v.wu16(dsWlogx1, 0)
	v.wu16(dsWlogy1, 0)
	v.wu16(dsWlogx2, 0x13f)
	v.wu16(dsWlogy2, 0xc7)
	v.clearWindow()
	v.wu32(dsCurSave, curSaveBuf)
}

// inisprit sets up the sprite segment (START.EXE 0xaeaa): the fixed
// sprites 2, 0x32, 0x62 and the mouse sprite 0x92, then the free list.
func (v *VM) inisprit() {
	for i := range uint32(0x100) {
		v.mem[0x1776f+i] = uint8(i + 1)
	}
	v.debsprit = v.ru32(dsDebsprit)
	v.wu32(dsFsBase, v.debsprit)
	v.setFs(v.debsprit)
	si := uint16(2)
	v.wu16(dsTvsprite, si)
	v.setFsu16(si, spNewx, 0)
	v.setFsu16(si, spNewy, 0)
	v.setFsu16(si, spDepx, 0x13f)
	v.setFsu16(si, spDepy, 0xc7)
	si += sprSize
	v.setFs8(si, spNumelem, 0)
	v.wu16(dsSprite32, si)
	si += sprSize
	v.wu16(dsSprite62, si)
	si += sprSize
	v.wu32(dsMouseAd, uint32(int32(int16(si)))+v.debsprit)
	v.wu16(dsMouseSpr, si)
	v.setFsu16(si, spNewx, 0)
	v.setFsu16(si, spNewy, 0)
	v.setFs8(si, spState, 0xfe)
	v.mem[dsMouseOn] = 0
	si += sprSize
	v.wu16(dsLibsprit, si)
	di := si
	for {
		si += sprSize
		if si > 0xffcf {
			break
		}
		v.setFsu16(di, spToNext, si)
		di = si
		if uint32(int32(int16(di)))+v.debsprit >= v.ru32(dsFinsprit) {
			break
		}
	}
	di -= sprSize
	v.setFsu16(di, spToNext, 0)
}
