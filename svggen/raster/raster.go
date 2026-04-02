// Package raster provides shared synchronization for rasterization operations.
//
// The tdewolff/canvas library has race conditions in its rasterizer — specifically
// in the bentleyOttmann path intersection algorithm (path_intersection.go) which
// uses package-level globals (_ps, _qs, _op, _fillRule at line 1784).
// All code that calls rasterizer.Draw() or canvas.RenderTo() must acquire Mu
// before doing so to avoid data races.
//
// Investigation (2026-03-08): Checked canvas@v0.0.0-20260307092048 (latest).
// The global state is still present. Per-instance rasterizers are not feasible
// because the globals are in the path intersection layer, not the rasterizer.
// Callers should keep png.Encode and pdfWriter.Close outside the lock scope
// since those operations are thread-safe.
//
// This package exists so that every call site across the codebase serializes on
// the same mutex, regardless of which package initiates the render.
package raster

import "sync"

// Mu serializes all tdewolff/canvas rasterization calls across the process.
// Acquire this mutex before calling rasterizer.Draw() or canvas.RenderTo().
var Mu sync.Mutex
