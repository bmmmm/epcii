//go:build js && wasm

// epcii-wasm is the browser build of epcii. It exposes the CLI pipeline to
// the page as globalThis.epcii.generate(fields) and stays resident; the
// static page lives in web/ and is assembled by scripts/build-web.sh.
package main

import (
	"runtime/debug"
	"syscall/js"

	"github.com/bmmmm/epcii/internal/webapi"
)

var version = "dev"

// generate maps a JS object with the eight string fields to webapi.Generate
// and returns a plain JS object. Missing or non-string properties read as "",
// so a partial form is a validation error from epc, never a JS exception.
func generate(_ js.Value, args []js.Value) any {
	var in webapi.Input
	if len(args) > 0 && args[0].Type() == js.TypeObject {
		get := func(key string) string {
			v := args[0].Get(key)
			if v.Type() != js.TypeString {
				return ""
			}
			return v.String()
		}
		in = webapi.Input{
			Name: get("name"), IBAN: get("iban"), BIC: get("bic"), Amount: get("amount"),
			Purpose: get("purpose"), Ref: get("ref"), Text: get("text"), Info: get("info"),
		}
	}
	out := webapi.Generate(in)
	res := map[string]any{
		"error":   out.Error,
		"payload": out.Payload,
		"version": out.Version,
		"size":    out.Size,
		"svg":     out.SVG,
	}
	png := js.Global().Get("Uint8Array").New(len(out.PNG))
	js.CopyBytesToJS(png, out.PNG)
	res["png"] = png
	return js.ValueOf(res)
}

// versionString mirrors main.go: an -ldflags override wins, then the module
// version from the build info, then "dev".
func versionString() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	js.Global().Set("epcii", js.ValueOf(map[string]any{
		"generate": js.FuncOf(generate),
		"version":  versionString(),
	}))
	select {}
}
