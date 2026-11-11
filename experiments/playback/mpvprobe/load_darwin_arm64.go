package mpvprobe

import (
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
	"my-bilibili/experiments/playback/contract"
)

func load(path string) (*nativeAPI, error) {
	info, err := os.Lstat(path)
	if !filepath.IsAbs(path) || err != nil || !info.Mode().IsRegular() {
		return nil, contract.Unsupported
	}
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, contract.Unsupported
	}
	a := &nativeAPI{unload: func() { _ = purego.Dlclose(h) }}
	bindings := []struct {
		name string
		fn   any
	}{
		{"mpv_client_api_version", &a.version}, {"mpv_create", &a.create},
		{"mpv_set_option_string", &a.option}, {"mpv_initialize", &a.initialize},
		{"mpv_observe_property", &a.observe}, {"mpv_command_async", &a.command},
		{"mpv_wait_event", &a.wait}, {"mpv_terminate_destroy", &a.destroy},
		{"mpv_render_context_create", &a.renderCreate}, {"mpv_render_context_update", &a.renderUpdate},
		{"mpv_render_context_render", &a.render}, {"mpv_render_context_free", &a.renderFree},
	}
	for _, binding := range bindings {
		symbol, err := purego.Dlsym(h, binding.name)
		if err != nil {
			a.unload()
			return nil, contract.Unsupported
		}
		purego.RegisterFunc(binding.fn, symbol)
	}
	if a.version() != 2<<16|6 {
		a.unload()
		return nil, contract.Unsupported
	}
	return a, nil
}
