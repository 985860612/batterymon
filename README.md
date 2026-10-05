# batterymon

Real-time power monitor for Windows laptops **and** desktops — battery charge/discharge watts, CPU package power (RAPL), live power chart, desktop GUI + CLI, Chinese/English UI.

[中文文档](README.zh-CN.md)

| Laptop (charging) | Desktop (no battery) |
|---|---|
| ![zh](docs/screenshot-zh.png) | ![en](docs/screenshot-en.png) |

## Features

- **Battery power (laptops)** — charge/discharge rate in watts, straight from the battery EC chip via WMI `root\wmi BatteryStatus`. No estimation.
- **CPU power (any modern Intel machine)** — package / cores / iGPU / DRAM watts via `\Energy Meter(RAPL_*)\Power` performance counters. **No driver, no admin rights needed.**
- **System & adapter input power** — on battery: exact (EC discharge rate = whole-system draw); while charging: estimated as charge + system, with the platform overhead (screen/SSD/Wi-Fi…) auto-calibrated from discharging periods.
- **Desktop mode** — no battery detected → the window switches to CPU-power-only view automatically.
- **Live chart** — 3-minute scrolling history of battery and CPU power.
- **Bilingual UI** — follows the Windows display language (中文/English), override with `-lang zh|en`.
- **CLI mode** — same data in a terminal, `-once` for scripts/alerts.
- **Tiny & standalone** — single ~9 MB exe, no installer, no .NET, no admin. GUI is pure Go ([Gio](https://gioui.org)), cross-compiled from macOS/Linux.

## Download / Build

Prebuilt binaries are on the [Releases](../../releases) page. Or build yourself (any OS with Go ≥ 1.23 cross-compiles; the WMI/PDH parts are Windows-only):

```sh
git clone https://github.com/985860612/batterymon.git
cd batterymon

# CLI
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o batterymon.exe .

# GUI — embed icon/version resource first (needs goversioninfo)
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
cd gui && goversioninfo -o rsrc_windows_amd64.syso versioninfo.json && cd ..
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o batterymon-gui.exe ./gui
```

## Usage

```sh
# GUI: double-click batterymon-gui.exe

# CLI: live view, refreshes every 2 s
batterymon.exe

# custom interval / single sample for scripts / force language
batterymon.exe -i 5s
batterymon.exe -once
batterymon.exe -once -lang en
```

Example CLI output (laptop, charging):

```
status      : AC / charging
charge power: 27.08 W  (into battery)
cpu package : 4.06 W  (cores 3.42 / igpu 0.01 / dram 0.00)
system (est): 8.06 W
ac in  (est): 35.14 W  (adapter output)
remaining   : 32.52 Wh  (44.4% / of full 73.2 Wh)
voltage     : 16.55 V
eta         : to full 1h 30m
```

## How it works

| Metric | Source | Accuracy |
|---|---|---|
| Charge/discharge power | Battery EC via WMI `root\wmi BatteryStatus` | Exact; discharge = whole-system power |
| CPU power | `\Energy Meter(RAPL_Package0_PKG/PP0/PP1/DRAM)\Power` perf counters (PDH API) | Exact (Intel RAPL) |
| System power | On battery: EC discharge (exact). Charging: RAPL + overhead (est.) | Mixed |
| AC adapter input | Not exposed by firmware on most laptops → estimated as charge + system | Estimate |

Notes / caveats:

- The `\Power Meter(*)\Power` counter exists on some machines but often reads 0 (unimplemented by firmware) — that would have been true adapter input.
- Rate-type PDH counters need two samples; the first reading after start is skipped.
- RAPL counters are Intel-specific (Sandy Bridge+). AMD machines currently get battery data only — patches welcome.

## Project layout

```
main.go            CLI (windows)
gui/main.go        GUI app (windows)
gui/ui/            pure UI layer, data-driven, cross-platform
gui/preview/       fake-data preview for UI iteration on macOS/Linux
internal/power/    battery EC (WMI) + RAPL (PDH) + overhead calibration
internal/i18n/     zh/en detection & strings
tools/icon/        programmatic icon generator (vector-drawn battery + bolt)
```

## License

[MIT](LICENSE)
