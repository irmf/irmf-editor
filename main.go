package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"syscall/js"
	"time"
)

var (
	editor      js.Value
	canvas      js.Value
	logfDiv     js.Value
	sliceButton js.Value
)

func main() {
	source := loadSource()

	// Wait until JS is initialized
	f := func() {
		editor = js.Global().Call("getEditor")
		doc := js.Global().Get("document")
		canvas = doc.Call("getElementById", "canvas")
		logfDiv = doc.Call("getElementById", "logf")
		sliceButton = doc.Call("getElementById", "slice-button")
	}
	f()
	for editor.Type() == js.TypeNull ||
		editor.Type() == js.TypeUndefined ||
		canvas.Type() == js.TypeNull ||
		logfDiv.Type() == js.TypeNull ||
		logfDiv.Type() == js.TypeUndefined ||
		sliceButton.Type() == js.TypeNull ||
		sliceButton.Type() == js.TypeUndefined {
		time.Sleep(100 * time.Millisecond)
		f()
	}

	// Install compileShader callback.
	cb := js.FuncOf(compileShader)
	v := js.Global().Get("installCompileShader")
	if v.Type() == js.TypeFunction {
		logf("Installing compileShader callback")
		v.Invoke(cb)
	}

	// // Install slice-button callback.
	// cb2 := js.FuncOf(sliceShader)
	// v2 := js.Global().Get("installSliceShader")
	// if v2.Type() == js.TypeFunction {
	// 	logf("Installing sliceShader callback")
	// 	v2.Invoke(cb2)
	// }

	if source != "" {
		initShader(source)
	} else {
		initShader(startupShader)
	}

	logf("Application irmf-editor is now started")

	// prevent program from terminating
	c := make(chan struct{}, 0)
	<-c
}

func compileShader(this js.Value, args []js.Value) interface{} {
	clearLog()
	logf("Go compileShader called!")
	src := editor.Call("getValue").String()
	return initShader(src)
}

func initShader(src string) interface{} {
	lines := strings.Split(src, "\n")
	if lines[0] != "/*{" {
		logf(`Unable to find leading "/*{"`) // TODO: Turn errors into hover-over text.
		js.Global().Call("highlightShaderError", 1)
		return nil
	}
	endJSON := strings.Index(src, "\n}*/\n")
	if endJSON < 0 {
		logf(`Unable to find trailing "}*/"`)
		// Try to find the end of the JSON blob.
		if lineNum := findKeyLine(src, "*/"); lineNum > 2 {
			js.Global().Call("highlightShaderError", lineNum)
			return nil
		}
		if lineNum := findKeyLine(src, "}*"); lineNum > 2 {
			js.Global().Call("highlightShaderError", lineNum)
			return nil
		}
		if lineNum := findKeyLine(src, "}"); lineNum > 2 {
			js.Global().Call("highlightShaderError", lineNum)
			return nil
		}
		js.Global().Call("highlightShaderError", 1)
		return nil
	}

	jsonBlobStr := src[2 : endJSON+2]
	// logf(jsonBlobStr)
	jsonBlob, err := parseJSON(jsonBlobStr)
	if err != nil {
		logf("Unable to parse JSON blob: %v", err)
		js.Global().Call("highlightShaderError", 2)
		return nil
	}

	shaderSrc := src[endJSON+5:]

	if lineNum, err := jsonBlob.validate(jsonBlobStr, shaderSrc); err != nil {
		logf("Invalid JSON blob: %v", err)
		js.Global().Call("highlightShaderError", lineNum)
		return nil
	}

	// TODO: Figure out how to preserve the cursor location on rewrite.
	// Rewrite the editor buffer:
	newShader, err := jsonBlob.format(shaderSrc)
	if err != nil {
		logf("Error: %v", err)
	} else {
		editor.Call("setValue", newShader)
	}

	// Set the updated MBB and number of materials:
	js.Global().Call("setMBB", jsonBlob.Min[0], jsonBlob.Min[1], jsonBlob.Min[2],
		jsonBlob.Max[0], jsonBlob.Max[1], jsonBlob.Max[2], len(jsonBlob.Materials))

	// logf("Compiling new model shader:\n%v", shaderSrc)
	js.Global().Call("loadNewModel", shaderSrc)

	return nil
}

func loadSource() string {
	const oldPrefix = "/?s=github.com/"
	const newPrefix = "https://raw.githubusercontent.com/"
	url := js.Global().Get("document").Get("location").String()
	i := strings.Index(url, oldPrefix)
	if i < 0 {
		logf("No source requested in URL path.")
		return ""
	}
	location := url[i+len(oldPrefix):]
	lower := strings.ToLower(location)
	if !strings.HasSuffix(lower, ".irmf") {
		js.Global().Call("alert", "irmf-editor will only load .irmf files")
		return ""
	}

	location = newPrefix + strings.Replace(location, "/blob/", "/", 1)

	resp, err := http.Get(location)
	if err != nil {
		logf("Unable to download source from: %v", location)
		js.Global().Call("alert", "unable to load IRMF shader")
		return ""
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logf("Unable to ready response body.")
		return ""
	}
	resp.Body.Close()
	logf("Read %v bytes from GitHub.", len(buf))
	return string(buf)
}

func clearLog() {
	if logfDiv.Type() != js.TypeNull {
		logfDiv.Set("innerHTML", "")
	}
}

func logf(fmtStr string, args ...interface{}) {
	if logfDiv.Type() != js.TypeNull && logfDiv.Type() != js.TypeUndefined {
		txt := logfDiv.Get("innerHTML").String()
		txt += fmt.Sprintf("<div>"+fmtStr+"</div>", args...)
		logfDiv.Set("innerHTML", txt)
	} else {
		fmt.Printf(fmtStr+"\n", args...)
	}
}

const startupShader = `/*{
  irmf: "1.0",
  materials: ["PLA"],
  max: [5,5,5],
  min: [-5,-5,-5],
  units: "mm",
}*/

float sphere(in vec3 pos, in float radius, in vec3 xyz) {
  xyz -= pos;  // Move sphere into place.
  float r = length(xyz);
  return r <= radius ? 1.0 : 0.0;
}

void mainModel4( out vec4 materials, in vec3 xyz ) {
  const float radius = 5.0;  // 10mm diameter sphere.
  materials[0] = sphere(vec3(0), radius, xyz);
}
`
