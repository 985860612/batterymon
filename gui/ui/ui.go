// Package ui 是 batterymon 的界面层: 纯数据驱动, 不碰 WMI/PDH, 跨平台可跑
// (Windows 正式版在 gui/ 里喂真实数据, Mac 上 gui/preview 喂假数据做视觉验证)
package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/985860612/batterymon/internal/i18n"
)

const HistoryCap = 180 // 曲线保留的采样点数 (1s 一个 = 3 分钟)

type ViewData struct {
	HasBattery bool // false = 台式机: 大数字显示 CPU 功率, 隐藏电池相关行
	BattW      float32 // 电池功率: 放电/充电均为正值, 状态看 Charging/Online
	Charging   bool
	Online     bool
	Critical   bool
	Err        error
	RAPLOK     bool
	PkgW       float32
	CoresW     float32
	IGPUW      float32
	DRAMW      float32
	SystemW    float32 // 整机系统功率
	SysExact   bool    // true=放电时 EC 精确值; false=充电时估算
	ShowACIn   bool    // 充电中才显示输入功率行
	ACInW      float32 // 适配器输入 (估算)
	RemainWh   float32
	Pct        float32
	VoltageV   float32
	ETA        string
	BattHist   []float32 // 电池功率曲线
	PkgHist    []float32 // CPU 功率曲线
	ChartHint  string    // 曲线左下角的量程标注前缀
}

var (
	colBg      = color.NRGBA{R: 0x12, G: 0x14, B: 0x18, A: 0xff}
	colFg      = color.NRGBA{R: 0xe6, G: 0xe6, B: 0xe6, A: 0xff}
	colDim     = color.NRGBA{R: 0x8a, G: 0x92, B: 0x9e, A: 0xff}
	colDischg  = color.NRGBA{R: 0x4f, G: 0xc3, B: 0x7a, A: 0xff} // 放电绿
	colChg     = color.NRGBA{R: 0x4f, G: 0xa3, B: 0xf5, A: 0xff} // 充电蓝
	colCrit    = color.NRGBA{R: 0xf0, G: 0x55, B: 0x4a, A: 0xff}
	colChartBg = color.NRGBA{R: 0x1a, G: 0x1e, B: 0x25, A: 0xff}
	colCPU     = color.NRGBA{R: 0xb8, G: 0x86, B: 0xf0, A: 0xff}
)

func label(th *material.Theme, size unit.Sp, txt string, col color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		l := material.Label(th, size, txt)
		l.Color = col
		return l.Layout(gtx)
	}
}

func row(th *material.Theme, k, v string, vcol color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(label(th, 13, k, colDim)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Rigid(label(th, 13, v, vcol)),
		)
	}
}

// drawChart 画双序列功率曲线: 主序列亮色描边+淡填充, 副序列细线; legend 为图例文字
func drawChart(gtx layout.Context, th *material.Theme, primary, secondary []float32, accent color.NRGBA, legend string) layout.Dimensions {
	h := gtx.Dp(110)
	w := gtx.Constraints.Max.X
	sz := image.Pt(w, h)

	rrect := clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(8))
	stack := rrect.Push(gtx.Ops)
	paint.Fill(gtx.Ops, colChartBg)
	stack.Pop()

	peak := float32(0)
	for _, v := range primary {
		peak = float32(math.Max(float64(peak), float64(v)))
	}
	for _, v := range secondary {
		peak = float32(math.Max(float64(peak), float64(v)))
	}
	scale := float32(math.Max(5, math.Ceil(float64(peak)*1.2)))
	base := float32(h) * 0.92
	yOf := func(v float32) float32 { return base - float32(h)*0.8*(v/scale) }
	step := float32(w) / float32(HistoryCap-1)

	plot := func(series []float32, col color.NRGBA, fill bool, widthDp unit.Dp) {
		if len(series) < 2 {
			return
		}
		if fill {
			var area clip.Path
			area.Begin(gtx.Ops)
			area.MoveTo(f32.Pt(0, base))
			for i, v := range series {
				area.LineTo(f32.Pt(step*float32(i), yOf(v)))
			}
			area.LineTo(f32.Pt(step*float32(len(series)-1), base))
			area.Close()
			fc := col
			fc.A = 0x2a
			paint.FillShape(gtx.Ops, fc, clip.Outline{Path: area.End()}.Op())
		}
		var line clip.Path
		line.Begin(gtx.Ops)
		for i, v := range series {
			p := f32.Pt(step*float32(i), yOf(v))
			if i == 0 {
				line.MoveTo(p)
			} else {
				line.LineTo(p)
			}
		}
		paint.FillShape(gtx.Ops, col, clip.Stroke{Path: line.End(), Width: float32(gtx.Dp(widthDp))}.Op())
	}

	plot(secondary, colCPU, false, 1) // CPU 细紫线
	plot(primary, accent, true, 1.5)  // 主线

	// 量程 + 图例
	stack2 := op.Offset(image.Pt(gtx.Dp(6), gtx.Dp(4))).Push(gtx.Ops)
	lbl := material.Label(th, 10, fmt.Sprintf("0-%.0fW", scale))
	lbl.Color = colDim
	lbl.Layout(gtx)
	stack2.Pop()

	lw := gtx.Dp(60)
	stack3 := op.Offset(image.Pt(w-lw-gtx.Dp(6), gtx.Dp(4))).Push(gtx.Ops)
	lg := material.Label(th, 10, legend)
	lg.Color = colDim
	lg.Layout(gtx)
	stack3.Pop()

	return layout.Dimensions{Size: sz}
}

// Layout 完整绘制一帧
func Layout(gtx layout.Context, th *material.Theme, v ViewData) {
	paint.Fill(gtx.Ops, colBg)

	accent := colDischg
	statusText := i18n.T("电池 · 放电中 (=整机功耗)", "Battery · discharging (= system power)")
	bigW := v.BattW
	chartPrimary, chartSecondary := v.BattHist, v.PkgHist
	legend := i18n.T("电池 | CPU", "batt | cpu")
	switch {
	case v.Err != nil:
		accent = colCrit
		statusText = i18n.T("查询失败", "query error")
	case !v.HasBattery:
		// 台式机: 大数字=CPU 封装功率, 曲线只画 CPU
		accent = colCPU
		statusText = i18n.T("台式机 · 电源供电(无电池) · CPU 封装功耗", "Desktop · AC powered (no battery) · CPU package")
		bigW = v.PkgW
		chartPrimary, chartSecondary = v.PkgHist, nil
		legend = "CPU"
	case v.Charging:
		accent = colChg
		statusText = i18n.T("电源 · 电池充电中", "AC · charging battery")
	case v.Online:
		accent = colChg
		statusText = i18n.T("电源已接 · 电池待机", "AC connected · battery idle")
	case v.Critical:
		accent = colCrit
		statusText = i18n.T("电池 · 电量危急!", "Battery · CRITICAL")
	}

	powerText := "--"
	if v.Err == nil {
		powerText = fmt.Sprintf("%.2f", bigW)
	}

	layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// 大数字: 电池功率(笔记本) 或 CPU 功率(台式机)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(label(th, 52, powerText, accent)),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(label(th, 18, "W", colDim)),
				)
			}),
			layout.Rigid(label(th, 12, statusText, accent)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			// 功率曲线
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawChart(gtx, th, chartPrimary, chartSecondary, accent, legend)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			// 功率明细
			layout.Rigid(rowIf(th, v.HasBattery, i18n.T("输入功率(估)", "AC in (est)"), fmtW(v.ACInW, v.ShowACIn), colFg)),
			layout.Rigid(rowIf(th, v.HasBattery, sysLabel(v.SysExact), fmtW(v.SystemW, true), colFg)),
			layout.Rigid(row(th, i18n.T("CPU 封装", "CPU pkg"), fmtW(v.PkgW, v.RAPLOK), colFg)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(row(th, i18n.T("  核心 / 核显", "  cores / iGPU"), fmt.Sprintf("%s / %s", fmtW(v.CoresW, v.RAPLOK), fmtW(v.IGPUW, v.RAPLOK)), colDim)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(row(th, i18n.T("  内存", "  DRAM"), fmtW(v.DRAMW, v.RAPLOK), colDim)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			// 电池明细 (台式机隐藏)
			layout.Rigid(rowIf(th, v.HasBattery, i18n.T("剩余电量", "Remaining"), fmt.Sprintf("%.2f Wh  (%.1f%%)", v.RemainWh, v.Pct), colFg)),
			layout.Rigid(rowIf(th, v.HasBattery, i18n.T("电压", "Voltage"), fmt.Sprintf("%.2f V", v.VoltageV), colFg)),
			layout.Rigid(rowIf(th, v.HasBattery, i18n.T("预计时间", "ETA"), v.ETA, colFg)),
			// 剩余空间吃掉, 防止内容被拉伸重叠
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
		)
	})
}

// rowIf 条件行: show=false 时不占空间
func rowIf(th *material.Theme, show bool, k, v string, vcol color.NRGBA) layout.Widget {
	if !show {
		return func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}
	return row(th, k, v, vcol)
}

func sysLabel(exact bool) string {
	if exact {
		return i18n.T("系统功耗(精确)", "System (exact)")
	}
	return i18n.T("系统功耗(估)", "System (est)")
}

func fmtW(v float32, ok bool) string {
	if !ok {
		return "--"
	}
	return fmt.Sprintf("%.2f W", v)
}
