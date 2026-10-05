// preview - 在 Mac 上用假数据预览 batterymon 界面, 供视觉迭代
// 用法: go run ./gui/preview
package main

import (
	"fmt"
	"math"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/985860612/batterymon/gui/ui"
	"github.com/985860612/batterymon/internal/i18n"
)

func fakeFrame(t int, battHist, pkgHist []float32, desktop bool) ui.ViewData {
	// 模拟充电中: 电池 27W 左右波动, CPU 2-20W 周期起伏
	batt := float32(27 + math.Sin(float64(t)/7)*1.5)
	pkg := float32(8 + 10*(0.5+0.5*math.Sin(float64(t)/11)))
	if desktop {
		return ui.ViewData{
			HasBattery: false,
			RAPLOK:     true,
			PkgW:       pkg,
			CoresW:     pkg * 0.82,
			IGPUW:      0.05,
			DRAMW:      1.2,
			PkgHist:    pkgHist,
		}
	}
	return ui.ViewData{
		HasBattery: true,
		BattW:      batt,
		Charging:   true,
		Online:     true,
		RAPLOK:     true,
		PkgW:       pkg,
		CoresW:     pkg * 0.82,
		IGPUW:      0.05,
		DRAMW:      0,
		SystemW:    pkg + 4.2,
		SysExact:   false,
		ShowACIn:   true,
		ACInW:      batt + pkg + 4.2,
		RemainWh:   20.34,
		Pct:        27.8,
		VoltageV:   16.22,
		ETA:        i18n.ETA(true, 53, 27),
		BattHist:   battHist,
		PkgHist:    pkgHist,
	}
}

func main() {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var battHist, pkgHist []float32
	desktop := len(os.Args) > 1 && os.Args[1] == "-desktop"
	for _, a := range os.Args[1:] {
		if a == "-zh" {
			i18n.Set("zh")
		}
	}
	v := fakeFrame(0, nil, nil, desktop)

	go func() {
		w := new(app.Window)
		w.Option(app.Title("batterymon preview"), app.Size(unit.Dp(340), unit.Dp(520)))
		var ops op.Ops
		t := 0
		// 预填 60 个历史点让曲线有形状
		for i := 0; i < 60; i++ {
			battHist = append(battHist, float32(27+math.Sin(float64(i-60)/7)*1.5))
			pkgHist = append(pkgHist, float32(8+10*(0.5+0.5*math.Sin(float64(i-60)/11))))
		}
		for {
			e := w.Event()
			switch e := e.(type) {
			case app.DestroyEvent:
				if e.Err != nil {
					fmt.Println("window error:", e.Err)
				}
				return
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				ui.Layout(gtx, th, v)
				e.Frame(gtx.Ops)
				t++
				battHist = append(battHist, float32(27+math.Sin(float64(t)/7)*1.5))
				pkgHist = append(pkgHist, float32(8+10*(0.5+0.5*math.Sin(float64(t)/11))))
				if len(battHist) > ui.HistoryCap {
					battHist = battHist[len(battHist)-ui.HistoryCap:]
					pkgHist = pkgHist[len(pkgHist)-ui.HistoryCap:]
				}
				v = fakeFrame(t, battHist, pkgHist, desktop)
				time.AfterFunc(time.Second, w.Invalidate)
			}
		}
	}()

	app.Main()
}
