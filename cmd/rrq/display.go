package main

import (
	"fmt"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
)

// display shows the 320x200 game screen, scaled to 4:3 like a VGA monitor
// and letterboxed in the window.
type display struct {
	device  *wgpu.Device
	queue   *wgpu.Queue
	surface *wgpu.Surface
	format  gputypes.TextureFormat
	w, h    int

	tex  *wgpu.Texture
	bg   *wgpu.BindGroup
	pipe *wgpu.RenderPipeline
	bgra bool
	px   []byte
}

const blitWGSL = `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
struct VOut { @builtin(position) pos: vec4f, @location(0) uv: vec2f }
@vertex
fn vs(@builtin(vertex_index) id: u32) -> VOut {
	let p = vec2f(f32((id << 1u) & 2u), f32(id & 2u));
	var o: VOut;
	o.uv = vec2f(p.x, 1.0 - p.y);
	o.pos = vec4f(p * 2.0 - 1.0, 0.0, 1.0);
	return o;
}
@fragment
fn fs(i: VOut) -> @location(0) vec4f { return textureSample(tex, samp, i.uv); }
`

func newDisplay(win *glfw.Window, sw, sh int) (*display, error) {
	d := &display{px: make([]byte, sw*sh*4)}
	inst, err := wgpu.CreateInstance(nil)
	if err != nil {
		return nil, fmt.Errorf("wgpu instance: %w", err)
	}
	if d.surface, err = inst.CreateSurfaceUnsafe(surfaceTarget(win)); err != nil {
		return nil, fmt.Errorf("wgpu surface: %w", err)
	}
	ad, err := inst.RequestAdapter(&wgpu.RequestAdapterOptions{CompatibleSurface: d.surface})
	if err != nil {
		return nil, fmt.Errorf("wgpu adapter: %w", err)
	}
	if d.device, err = ad.RequestDevice(&wgpu.DeviceDescriptor{Label: "rrq"}); err != nil {
		return nil, fmt.Errorf("wgpu device: %w", err)
	}
	d.queue = d.device.Queue()
	d.format = gputypes.TextureFormatBGRA8Unorm
	if caps := ad.GetSurfaceCapabilities(d.surface); caps != nil && len(caps.Formats) > 0 {
		d.format = caps.Formats[0]
		for _, f := range caps.Formats {
			if f == gputypes.TextureFormatBGRA8Unorm || f == gputypes.TextureFormatRGBA8Unorm {
				d.format = f
				break
			}
		}
	}

	if d.tex, err = d.device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "screen", Size: wgpu.Extent3D{Width: uint32(sw), Height: uint32(sh), DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D,
		Format: gputypes.TextureFormatBGRA8Unorm,
		Usage:  gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	}); err != nil {
		return nil, err
	}
	view, err := d.device.CreateTextureView(d.tex, nil)
	if err != nil {
		return nil, err
	}
	samp, err := d.device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "screen",
		AddressModeU: gputypes.AddressModeClampToEdge, AddressModeV: gputypes.AddressModeClampToEdge, AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter: gputypes.FilterModeNearest, MinFilter: gputypes.FilterModeNearest, MipmapFilter: gputypes.FilterModeNearest,
		LodMaxClamp: 1,
	})
	if err != nil {
		return nil, err
	}
	layout, err := d.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "screen",
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		},
	})
	if err != nil {
		return nil, err
	}
	if d.bg, err = d.device.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "screen", Layout: layout, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, TextureView: view}, {Binding: 1, Sampler: samp},
	}}); err != nil {
		return nil, err
	}
	mod, err := d.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "blit", WGSL: blitWGSL})
	if err != nil {
		return nil, err
	}
	pl, err := d.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "blit", BindGroupLayouts: []*wgpu.BindGroupLayout{layout}})
	if err != nil {
		return nil, err
	}
	if d.pipe, err = d.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:       "blit",
		Layout:      pl,
		Vertex:      wgpu.VertexState{Module: mod, EntryPoint: "vs"},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{Module: mod, EntryPoint: "fs", Targets: []gputypes.ColorTargetState{
			{Format: d.format, WriteMask: gputypes.ColorWriteMaskAll}}},
	}); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *display) resize(w, h int) error {
	if w == d.w && h == d.h || w == 0 || h == 0 {
		return nil
	}
	if err := d.surface.Configure(d.device, &wgpu.SurfaceConfiguration{
		Width: uint32(w), Height: uint32(h), Format: d.format,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: gputypes.PresentModeFifo, AlphaMode: gputypes.CompositeAlphaModeOpaque,
	}); err != nil {
		return err
	}
	d.w, d.h = w, h
	return nil
}

// draw uploads argb (0xAARRGGBB, sw x sh) and presents it.
func (d *display) draw(argb []uint32, sw, sh int) error {
	if d.w == 0 {
		return nil
	}
	for i, p := range argb {
		d.px[i*4], d.px[i*4+1], d.px[i*4+2], d.px[i*4+3] = byte(p), byte(p>>8), byte(p>>16), 0xff
	}
	if err := d.queue.WriteTexture(&wgpu.ImageCopyTexture{Texture: d.tex}, d.px,
		&wgpu.ImageDataLayout{BytesPerRow: uint32(sw * 4), RowsPerImage: uint32(sh)},
		&wgpu.Extent3D{Width: uint32(sw), Height: uint32(sh), DepthOrArrayLayers: 1}); err != nil {
		return err
	}
	st, _, err := d.surface.GetCurrentTexture()
	if err != nil {
		d.w = 0 // reconfigure
		return nil
	}
	target, err := st.CreateView(nil)
	if err != nil {
		return err
	}
	defer target.Release()
	enc, err := d.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "frame"})
	if err != nil {
		return err
	}
	p, err := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{Label: "screen",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{View: target, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore}}})
	if err != nil {
		return err
	}
	// 4:3 letterbox
	vw, vh := float32(d.w), float32(d.w)*3/4
	if vh > float32(d.h) {
		vh = float32(d.h)
		vw = vh * 4 / 3
	}
	p.SetViewport(gputypes.Viewport{X: (float32(d.w) - vw) / 2, Y: (float32(d.h) - vh) / 2, Width: vw, Height: vh, MaxDepth: 1})
	p.SetPipeline(d.pipe)
	p.SetBindGroup(0, d.bg, nil)
	p.Draw(gputypes.DrawArgs{VertexCount: 3, InstanceCount: 1})
	if err := p.End(); err != nil {
		return err
	}
	cmd, err := enc.Finish()
	if err != nil {
		return err
	}
	if _, err := d.queue.Submit(cmd); err != nil {
		return err
	}
	if err := d.surface.Present(st); err != nil {
		d.w = 0
	}
	return nil
}
