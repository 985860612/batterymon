//go:build windows

// Package power 读取 Windows 笔记本的实时功率数据:
//   - 电池 EC: WMI root\wmi BatteryStatus (充/放电功率, EC 芯片直报)
//   - CPU RAPL: \Energy Meter(*)\Power 性能计数器 (PDH API, 无需驱动/admin)
//   - 整机/输入功率: 放电时整机=EC 放电率(精确); 充电时 输入=充电+系统(估算), 杂项功耗用放电态自动校准
package power

import (
	"syscall"
	"unsafe"

	"github.com/StackExchange/wmi"

	"github.com/985860612/batterymon/internal/i18n"
)

// ---------- 电池 EC ----------

// 注意: 结构体名必须与 WMI 类名一致, StackExchange/wmi 按类型名生成 SELECT * FROM <名字>
type BatteryStatus struct {
	PowerOnline       bool
	Charging          bool
	Discharging       bool
	Critical          bool
	ChargeRate        uint32 // mW
	DischargeRate     uint32 // mW
	RemainingCapacity uint32 // mWh
	Voltage           uint32 // mV
}

type BatteryFullChargedCapacity struct {
	FullChargedCapacity uint32 // mWh
}

type Battery struct {
	PowerOnline   bool
	Charging      bool
	Discharging   bool
	Critical      bool
	ChargeW       float64 // 充电功率
	DischargeW    float64 // 放电功率 (放电时=整机系统功率, 精确)
	RemainingWh   float64
	FullChargedWh float64 // 老化后的真实满充容量
	VoltageV      float64
}

func (b Battery) Pct() float64 {
	if b.FullChargedWh <= 0 {
		return 0
	}
	return b.RemainingWh / b.FullChargedWh * 100
}

// ETA 返回预计剩余时间文本: 放电=还能用多久, 充电=还要充多久 (中英随 i18n)
func (b Battery) ETA() string {
	if b.Charging {
		return i18n.ETA(true, b.FullChargedWh-b.RemainingWh, b.ChargeW)
	}
	return i18n.ETA(false, b.RemainingWh, b.DischargeW)
}

type Win32_Battery struct { //nolint:revive // 结构体名必须与 WMI 类名一致
	Name string
}

func ReadBattery() (Battery, bool, error) {
	// 先用 root\cimv2 的 Win32_Battery 探测有无电池:
	// 该类在无电池的台式机上也已注册(0 实例), 而 root\wmi 的 BatteryStatus
	// 在无电池机器上整个类不存在(查询报"无效类"), 不能直接拿来探测
	var probe []Win32_Battery
	if err := wmi.Query(wmi.CreateQuery(&probe, ""), &probe); err != nil {
		return Battery{}, false, err
	}
	if len(probe) == 0 {
		return Battery{}, false, nil // 台式机无电池, 非错误
	}
	var bs []BatteryStatus
	if err := wmi.QueryNamespace(wmi.CreateQuery(&bs, ""), &bs, `root\wmi`); err != nil {
		return Battery{}, false, err
	}
	if len(bs) == 0 {
		return Battery{}, false, nil
	}
	var fc []BatteryFullChargedCapacity
	_ = wmi.QueryNamespace(wmi.CreateQuery(&fc, ""), &fc, `root\wmi`)
	var full uint32
	for _, f := range fc {
		full += f.FullChargedCapacity
	}
	s := bs[0]
	return Battery{
		PowerOnline:   s.PowerOnline,
		Charging:      s.Charging,
		Discharging:   s.Discharging,
		Critical:      s.Critical,
		ChargeW:       float64(s.ChargeRate) / 1000,
		DischargeW:    float64(s.DischargeRate) / 1000,
		RemainingWh:   float64(s.RemainingCapacity) / 1000,
		FullChargedWh: float64(full) / 1000,
		VoltageV:      float64(s.Voltage) / 1000,
	}, true, nil
}

// ---------- RAPL (PDH 性能计数器) ----------

var (
	pdhDLL              = syscall.NewLazyDLL("pdh.dll")
	procPdhOpenQuery    = pdhDLL.NewProc("PdhOpenQueryW")
	procPdhAddCounter   = pdhDLL.NewProc("PdhAddEnglishCounterW")
	procPdhCollect      = pdhDLL.NewProc("PdhCollectQueryData")
	procPdhFmtValue     = pdhDLL.NewProc("PdhGetFormattedCounterValue")
)

const pdhFmtDouble = 0x00000200

type pdhFmtCounterValue struct {
	cStatus uint32
	_       uint32
	double  float64
}

type RAPL struct {
	PkgW, CoresW, IGPUW, DRAMW float64
	OK                         bool
}

type RAPLReader struct {
	query  uintptr
	cnt    [4]uintptr // pkg, pp0(cores), pp1(igpu), dram
	valid  [4]bool
	cycles int
}

func NewRAPLReader() *RAPLReader {
	r := &RAPLReader{}
	ret, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&r.query)))
	if ret != 0 {
		return r
	}
	paths := [4]string{
		`\Energy Meter(RAPL_Package0_PKG)\Power`,
		`\Energy Meter(RAPL_Package0_PP0)\Power`,
		`\Energy Meter(RAPL_Package0_PP1)\Power`,
		`\Energy Meter(RAPL_Package0_DRAM)\Power`,
	}
	for i, p := range paths {
		ptr, err := syscall.UTF16PtrFromString(p)
		if err != nil {
			continue
		}
		ret, _, _ := procPdhAddCounter.Call(r.query, uintptr(unsafe.Pointer(ptr)), 0, uintptr(unsafe.Pointer(&r.cnt[i])))
		r.valid[i] = ret == 0
	}
	// 速率型计数器需要两次采集才有值, 先空采一次
	procPdhCollect.Call(r.query)
	return r
}

func (r *RAPLReader) Sample() RAPL {
	var out RAPL
	if r.query == 0 {
		return out
	}
	ret, _, _ := procPdhCollect.Call(r.query)
	if ret != 0 {
		return out
	}
	r.cycles++
	if r.cycles < 2 {
		return out // 第一次采集无速率值
	}
	read := func(i int) float64 {
		if !r.valid[i] {
			return 0
		}
		var v pdhFmtCounterValue
		ret, _, _ := procPdhFmtValue.Call(r.cnt[i], pdhFmtDouble, 0, uintptr(unsafe.Pointer(&v)))
		if ret != 0 || v.cStatus != 0 {
			return 0
		}
		w := v.double / 1000 // cooked 值为 mW
		if w < 0 || w > 500 {  // 单位异常保护
			return 0
		}
		return w
	}
	out.PkgW, out.CoresW, out.IGPUW, out.DRAMW = read(0), read(1), read(2), read(3)
	out.OK = r.valid[0]
	return out
}

// ---------- 组合监视器 (含杂项功耗自校准) ----------

type Reading struct {
	HasBattery  bool // false = 台式机, 只有 RAPL 可用
	Batt        Battery
	RAPL        RAPL
	SystemW     float64 // 整机系统功率 (放电=EC 精确值; 充电=PKG+DRAM+杂项 估算; 无电池时无效)
	SystemExact bool
	ACInW       float64 // 适配器输入功率 (仅充电时有意义, 估算)
	OverheadW   float64 // 当前采用的杂项功耗 (屏幕/SSD/WiFi 等)
	Err         error
}

type Monitor struct {
	rapl        *RAPLReader
	overhead    float64 // EMA
	hasOverhead bool
}

func NewMonitor() *Monitor {
	return &Monitor{rapl: NewRAPLReader(), overhead: 4}
}

func (m *Monitor) Sample() Reading {
	batt, hasBatt, err := ReadBattery()
	rapl := m.rapl.Sample()
	rd := Reading{HasBattery: hasBatt, Batt: batt, RAPL: rapl, Err: err, OverheadW: m.overhead}
	if err != nil || !hasBatt {
		return rd
	}

	switch {
	case batt.Discharging && batt.DischargeW > 0:
		rd.SystemW = batt.DischargeW
		rd.SystemExact = true
		if rapl.OK {
			// 放电时整机功耗精确可知, 反推杂项功耗并做指数平滑
			oh := batt.DischargeW - rapl.PkgW - rapl.DRAMW
			if oh >= 0.5 && oh <= 30 {
				if !m.hasOverhead {
					m.overhead = oh
					m.hasOverhead = true
				} else {
					m.overhead = m.overhead*0.8 + oh*0.2
				}
				rd.OverheadW = m.overhead
			}
		}
	case batt.Charging:
		rd.SystemW = rapl.PkgW + rapl.DRAMW + m.overhead
		rd.ACInW = batt.ChargeW + rd.SystemW
	default:
		rd.SystemW = rapl.PkgW + rapl.DRAMW + m.overhead
	}
	return rd
}
