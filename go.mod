module github.com/retro-ware/robinson-go

go 1.25.0

replace github.com/gogpu/wgpu => github.com/memmaker/wgpu v0.0.0-20261023072902-870db13ffc75

require (
	github.com/ebitengine/oto/v3 v3.3.3
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20250301202403-da16c1255728
	github.com/gogpu/gputypes v0.8.0
	github.com/gogpu/wgpu v0.34.5
	golang.org/x/sys v0.47.0
)

require (
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gpucontext v0.31.3 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
)
