//go:build windows

// batterymon - 实时查看 Windows 笔记本/台式机功率
// 电池: WMI EC 直报; CPU: RAPL 性能计数器; 整机/输入: 放电精确 / 充电估算
// 语言: 跟随系统 UI 语言, -lang zh|en 可强制
package main

import (
	"flag"
	"fmt"
	"syscall"
	"time"

	"github.com/985860612/batterymon/internal/i18n"
	"github.com/985860612/batterymon/internal/power"
)

// setConsoleUTF8 把控制台输出码页切到 UTF-8, 避免本地 GBK 控制台中文乱码
// (对 SSH/管道输出无影响)
func setConsoleUTF8() {
	p := syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleOutputCP")
	p.Call(65001)
}

func render(rd power.Reading) {
	fmt.Print("\033[H\033[2J")
	fmt.Printf("batterymon  %s  %s\n", time.Now().Format("15:04:05"), i18n.T("(Ctrl+C 退出)", "(Ctrl+C quit)"))
	fmt.Println("----------------------------------------")
	if rd.Err != nil {
		fmt.Println(i18n.T("查询失败", "query failed")+":", rd.Err)
		return
	}
	if !rd.HasBattery {
		// 台式机: 只有 RAPL
		fmt.Println(i18n.T("状态    : 台式机 / 电源供电(无电池)", "status  : desktop / AC powered (no battery)"))
		if rd.RAPL.OK {
			fmt.Printf("%s: %.2f W  (%s %.2f / %s %.2f / %s %.2f)\n",
				i18n.T("CPU 封装", "cpu package"), rd.RAPL.PkgW,
				i18n.T("核心", "cores"), rd.RAPL.CoresW,
				i18n.T("核显", "igpu"), rd.RAPL.IGPUW,
				i18n.T("内存", "dram"), rd.RAPL.DRAMW)
		} else {
			fmt.Println(i18n.T("CPU 封装: -- (RAPL 计数器不可用)", "cpu package: -- (RAPL counter unavailable)"))
		}
		return
	}
	b := rd.Batt
	var status string
	switch {
	case b.Charging:
		status = i18n.T("电源 / 充电中", "AC / charging")
	case b.Discharging:
		status = i18n.T("电池 / 放电中", "battery / discharging")
	case b.PowerOnline:
		status = i18n.T("电源 / 电池待机", "AC / battery idle")
	default:
		status = i18n.T("电池 / 待机", "battery / idle")
	}
	if b.Critical {
		status += i18n.T("  [电量危急]", "  [CRITICAL]")
	}
	fmt.Printf("%s: %s\n", i18n.T("状态      ", "status      "), status)
	if b.Charging {
		fmt.Printf("%s: %.2f W  %s\n", i18n.T("充电功率", "charge power"), b.ChargeW, i18n.T("(充入电池)", "(into battery)"))
	} else {
		fmt.Printf("%s: %.2f W  %s\n", i18n.T("电池功率", "batt power  "), b.DischargeW, i18n.T("(放电)", "(discharge)"))
	}
	if rd.RAPL.OK {
		fmt.Printf("%s: %.2f W  (%s %.2f / %s %.2f / %s %.2f)\n",
			i18n.T("CPU 封装", "cpu package"), rd.RAPL.PkgW,
			i18n.T("核心", "cores"), rd.RAPL.CoresW,
			i18n.T("核显", "igpu"), rd.RAPL.IGPUW,
			i18n.T("内存", "dram"), rd.RAPL.DRAMW)
	}
	if b.Charging {
		fmt.Printf("%s: %.2f W\n", i18n.T("系统功耗(估)", "system (est)"), rd.SystemW)
		fmt.Printf("%s: %.2f W  %s\n", i18n.T("输入功率(估)", "ac in  (est)"), rd.ACInW, i18n.T("(适配器输出)", "(adapter output)"))
	} else if rd.SystemExact {
		fmt.Printf("%s: %.2f W  %s\n", i18n.T("系统功耗    ", "system      "), rd.SystemW, i18n.T("(精确, =电池放电)", "(exact, = batt discharge)"))
	} else {
		fmt.Printf("%s: %.2f W\n", i18n.T("系统功耗(估)", "system (est)"), rd.SystemW)
	}
	fmt.Printf("%s: %.2f Wh  (%.1f%% %s %.1f Wh)\n",
		i18n.T("剩余电量", "remaining   "), b.RemainingWh, b.Pct(), i18n.T("/ 满充", "/ of full"), b.FullChargedWh)
	fmt.Printf("%s: %.2f V\n", i18n.T("电压      ", "voltage     "), b.VoltageV)
	fmt.Printf("%s: %s\n", i18n.T("预计时间  ", "eta         "), b.ETA())
}

func main() {
	interval := flag.Duration("i", 2*time.Second, i18n.T("采样间隔, 如 1s 5s", "sampling interval, e.g. 1s 5s"))
	once := flag.Bool("once", false, i18n.T("输出一条后退出", "print one sample and exit"))
	lang := flag.String("lang", "", i18n.T("强制语言 zh|en (默认跟随系统)", "force language zh|en (default: system)"))
	flag.Parse()

	if *lang != "" {
		i18n.Set(*lang)
	}
	setConsoleUTF8()

	mon := power.NewMonitor()
	// RAPL 速率计数器需两次采集, 先预热一次
	mon.Sample()
	time.Sleep(300 * time.Millisecond)

	if *once {
		render(mon.Sample())
		return
	}
	for {
		render(mon.Sample())
		time.Sleep(*interval)
	}
}
