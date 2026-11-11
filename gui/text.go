package main

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// The Go side (tray menu, tooltip) reads its text from the same resource file
// as the frontend, so all UI text stays in one place.
//
//go:embed frontend/src/i18n/zh-CN.json
var zhCN []byte

var messages = func() map[string]any {
	var m map[string]any
	if err := json.Unmarshal(zhCN, &m); err != nil {
		panic("gui: bad zh-CN.json: " + err.Error())
	}
	return m
}()

// text looks up a dotted key such as "tray.quit". A missing key returns the
// key itself so it is noticed, like t() in the frontend.
func text(key string) string {
	var node any = messages
	for _, part := range strings.Split(key, ".") {
		m, ok := node.(map[string]any)
		if !ok {
			return key
		}
		node = m[part]
	}
	if s, ok := node.(string); ok {
		return s
	}
	return key
}
