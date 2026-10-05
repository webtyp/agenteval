//go:build !wasm

package ui

// wake does nothing outside the browser: there is no Worker to start.
func (p *Panel) wake() {}
