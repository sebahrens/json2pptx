// Package core provides the foundational types, interfaces, and registry for
// svggen diagram generation. Import this package for lightweight access to
// request/response types and the diagram registry without linking any diagram
// implementations.
//
// For full functionality with all built-in diagrams auto-registered, import
// the parent package github.com/sebahrens/json2pptx/svggen instead.
//
// Selective diagram registration:
//
//	import (
//	    "github.com/sebahrens/json2pptx/svggen/core"
//	    _ "github.com/sebahrens/json2pptx/svggen/diagrams/bar"  // register only bar charts
//	)
//
//	result, err := core.Render(&core.RequestEnvelope{...})
package core
