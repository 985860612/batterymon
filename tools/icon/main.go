// icon 生成器: 纯 Go 画 batterymon 图标 (蓝色圆角电池 + 白色闪电) 并写出多尺寸 .ico
// 用法: go run ./tools/icon   →  gui/icon.ico
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

var (
	colBody = color.NRGBA{R: 0x4f, G: 0xa3, B: 0xf5, A: 0xff} // 充电蓝
	colBolt = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff} // 白闪电
)

const master = 256

// rrect 用 vector 光栅化填充圆角矩形
func rrect(dst *image.RGBA, x, y, w, h, r float32, col color.NRGBA) {
	var v vector.Rasterizer
	v.Reset(master, master)
	v.DrawOp = draw.Src
	// 四边 + 四角圆弧近似 (直接用 vector 的圆角路径)
	v.MoveTo(x+r, y)
	v.LineTo(x+w-r, y)
	v.CubeTo(x+w-r/2, y, x+w, y+r/2, x+w, y+r) // 右上
	v.LineTo(x+w, y+h-r)
	v.CubeTo(x+w, y+h-r/2, x+w-r/2, y+h, x+w-r, y+h) // 右下
	v.LineTo(x+r, y+h)
	v.CubeTo(x+r/2, y+h, x, y+h-r/2, x, y+h-r) // 左下
	v.LineTo(x, y+r)
	v.CubeTo(x, y+r/2, x+r/2, y, x+r, y) // 左上
	v.ClosePath()
	v.Draw(dst, dst.Bounds(), image.NewUniform(col), image.Point{})
}

func poly(dst *image.RGBA, pts [][2]float32, col color.NRGBA) {
	var v vector.Rasterizer
	v.Reset(master, master)
	v.DrawOp = draw.Src
	v.MoveTo(pts[0][0], pts[0][1])
	for _, p := range pts[1:] {
		v.LineTo(p[0], p[1])
	}
	v.ClosePath()
	v.Draw(dst, dst.Bounds(), image.NewUniform(col), image.Point{})
}

func render(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, master, master))
	// 电池主体: 圆角矩形
	rrect(img, 26, 82, 174, 92, 22, colBody)
	// 电池正极凸头
	rrect(img, 204, 110, 26, 36, 8, colBody)
	// 闪电 (镂空感的白色实心)
	poly(img, [][2]float32{
		{150, 94}, {96, 140}, {126, 140}, {108, 178},
		{168, 124}, {136, 124}, {158, 94},
	}, colBolt)

	if size == master {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(out, out.Bounds(), img, img.Bounds(), draw.Over, nil)
	return out
}

// writeICO 写出 PNG 内嵌式 .ico (Vista+ 支持)
func writeICO(path string, sizes []int) error {
	type entry struct {
		img *image.RGBA
		png []byte
	}
	var entries []entry
	for _, s := range sizes {
		img := render(s)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		entries = append(entries, entry{img, buf.Bytes()})
	}

	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint16(0))             // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1))             // type: icon
	binary.Write(&out, binary.LittleEndian, uint16(len(entries)))  // count
	offset := uint32(6 + 16*len(entries))
	for _, e := range entries {
		w, h := byte(e.img.Bounds().Dx()), byte(e.img.Bounds().Dy())
		if e.img.Bounds().Dx() >= 256 {
			w = 0 // 256 记为 0
		}
		if e.img.Bounds().Dy() >= 256 {
			h = 0
		}
		out.WriteByte(w)                            // width
		out.WriteByte(h)                            // height
		out.WriteByte(0)                            // palette
		out.WriteByte(0)                            // reserved
		binary.Write(&out, binary.LittleEndian, uint16(1))          // planes
		binary.Write(&out, binary.LittleEndian, uint16(32))         // bpp
		binary.Write(&out, binary.LittleEndian, uint32(len(e.png))) // size
		binary.Write(&out, binary.LittleEndian, offset)             // offset
		offset += uint32(len(e.png))
	}
	for _, e := range entries {
		out.Write(e.png)
	}
	return os.WriteFile(path, out.Bytes(), 0644)
}

func main() {
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	if err := writeICO("gui/icon.ico", sizes); err != nil {
		fmt.Fprintln(os.Stderr, "write ico:", err)
		os.Exit(1)
	}
	// 顺手出一张 256 PNG 便于预览检查
	f, _ := os.Create("gui/icon-256.png")
	defer f.Close()
	png.Encode(f, render(256))
	info, _ := os.Stat("gui/icon.ico")
	fmt.Printf("gui/icon.ico written (%d bytes, sizes %v)\n", info.Size(), sizes)
}
