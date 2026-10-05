//go:build windows

// batterymon-gui - win2 实时功率桌面小窗
// 电池功率: WMI EC 直报; CPU: RAPL 性能计数器; 输入/整机: 放电精确/充电估算
package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/985860612/batterymon/gui/ui"
	"github.com/985860612/batterymon/internal/i18n"
	"github.com/985860612/batterymon/internal/power"
)

const sampleInterval = time.Second

type state struct {
	mu       sync.Mutex
	v        ui.ViewData
	battHist []float32
	pkgHist  []float32
}

func (st *state) push(rd power.Reading) {
	st.mu.Lock()
	defer st.mu.Unlock()
	v := &st.v
	v.Err = rd.Err
	if rd.Err == nil {
		v.HasBattery = rd.HasBattery
		v.BattW = float32(rd.Batt.DischargeW)
		v.Charging = rd.Batt.Charging
		v.Online = rd.Batt.PowerOnline
		v.Critical = rd.Batt.Critical
		if rd.Batt.Charging {
			v.BattW = float32(rd.Batt.ChargeW)
		}
		v.RemainWh = float32(rd.Batt.RemainingWh)
		v.Pct = float32(rd.Batt.Pct())
		v.VoltageV = float32(rd.Batt.VoltageV)
		v.ETA = rd.Batt.ETA()
		v.SystemW = float32(rd.SystemW)
		v.SysExact = rd.SystemExact
		v.ShowACIn = rd.HasBattery && rd.Batt.Charging
		v.ACInW = float32(rd.ACInW)
		v.RAPLOK = rd.RAPL.OK
		v.PkgW = float32(rd.RAPL.PkgW)
		v.CoresW = float32(rd.RAPL.CoresW)
		v.IGPUW = float32(rd.RAPL.IGPUW)
		v.DRAMW = float32(rd.RAPL.DRAMW)

		st.battHist = append(st.battHist, v.BattW)
		st.pkgHist = append(st.pkgHist, v.PkgW)
		if len(st.battHist) > ui.HistoryCap {
			st.battHist = st.battHist[len(st.battHist)-ui.HistoryCap:]
			st.pkgHist = st.pkgHist[len(st.pkgHist)-ui.HistoryCap:]
		}
		v.BattHist = st.battHist
		v.PkgHist = st.pkgHist
	}
}

func (st *state) snapshot() ui.ViewData {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.v
}

func main() {
	lang := flag.String("lang", "", "force language zh|en (default: system)")
	flag.Parse()
	if *lang != "" {
		i18n.Set(*lang)
	}

	st := &state{}
	mon := power.NewMonitor()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	winReady := make(chan *app.Window, 1)
	go func() {
		w := <-winReady
		for {
			st.push(mon.Sample())
			w.Invalidate()
			time.Sleep(sampleInterval)
		}
	}()

	go func() {
		host, _ := os.Hostname()
		w := new(app.Window)
		w.Option(
			app.Title("batterymon - "+host),
			app.Size(unit.Dp(340), unit.Dp(520)),
			app.MinSize(unit.Dp(300), unit.Dp(480)),
		)
		winReady <- w
		var ops op.Ops
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
				ui.Layout(gtx, th, st.snapshot())
				e.Frame(gtx.Ops)
			}
		}
	}()

	app.Main()
}
