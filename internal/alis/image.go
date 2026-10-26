package alis

// image is the display state (alis sImage, PC fields only).
type image struct {
	landImage
	spritemem []byte
	physic    []byte
	logic     []byte
	backmap   []byte // background cache, nil when off

	logx1, logx2, logy1, logy2 int16
	loglarg                    int16

	numelem int8
	invertX uint8

	depx, depy, depz       int16
	oldcx, oldcy, oldcz    int16
	oldacx, oldacy, oldacz int16

	dkpalet  [1024]int16
	fdarkpal uint8
	mpalet   [256]uint32 // live palette (ARGB)
	dpal     dosPal

	flinepal uint8
	tlinepal [64]int16

	backdes, backaddr              int32
	backprof                       int16
	backx1, backx2, backy1, backy2 int16
	backlarg                       int16

	basesprite int32
	libsprit   uint16
	debsprit   int32
	finsprit   int32
	backsprite int32
	tvsprite   int32
	texsprite  int32
	atexsprite int32

	spag, wpag                 int8
	pagdx, pagdy, pagcount     int16
	sback, wback               uint8
	cback                      int8
	pback                      uint8
	inkcolor, lineAMode        uint16
	tabfen                     [640]int16
	ptabfen                    int
	fmouse                     int8
	fonum                      int16
	newad                      uint32
	newx, newy, newd, newf     int16
	newh, newl                 int16
	newzoomx, newzoomy         int16
	blocx1, blocy1             int16
	blocx2, blocy2             int16
	joints                     uint8
	fenx1, feny1, fenx2, feny2 int16
	fenlargw                   int16
	clipx1, clipy1             int16
	clipx2, clipy2             int16
	clipl, cliph               int16
	fclip                      uint8

	fitroutine, fphysic, fphytolog uint8
	fremap                         uint8
	vtiming                        uint8
	switchgo                       uint8
	wlogicBack                     bool
	wlogx1, wlogx2, wlogy1, wlogy2 int16
	wloglarg                       int16

	fdoland  uint8
	scdirect int16

	atlland  uint32
	zoombid  uint32
	landdata uint32
}
