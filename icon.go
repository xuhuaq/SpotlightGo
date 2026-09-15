// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

func trayIcon() []byte {
	const size = 64
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)
	// A small Spotlight-like lamp: warm disc plus blue download arrow.
	for y := 6; y < 58; y++ {
		for x := 6; x < 58; x++ {
			dx, dy := x-32, y-32
			if dx*dx+dy*dy <= 26*26 {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 193, B: 45, A: 255})
			}
		}
	}
	blue := color.NRGBA{R: 55, G: 156, B: 220, A: 255}
	for y := 15; y < 38; y++ {
		for x := 27; x < 37; x++ {
			img.SetNRGBA(x, y, blue)
		}
	}
	for row := 0; row < 14; row++ {
		for x := 20 + row; x <= 44-row; x++ {
			img.SetNRGBA(x, 34+row, blue)
		}
	}
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, img)
	return pngToICO(pngData.Bytes(), size, size)
}

func pngToICO(pngData []byte, width, height int) []byte {
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	out.WriteByte(byte(width))
	out.WriteByte(byte(height))
	out.WriteByte(0)
	out.WriteByte(0)
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(32))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(pngData)))
	_ = binary.Write(&out, binary.LittleEndian, uint32(22))
	out.Write(pngData)
	return out.Bytes()
}
