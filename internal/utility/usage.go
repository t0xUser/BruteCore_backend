package utility

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

func GetRAMUsage() *string {
	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil
	}

	usedPercent := memory.UsedPercent
	usedGB := float64(memory.Used) / (1 << 30)
	totalGB := float64(memory.Total) / (1 << 30)
	res := fmt.Sprintf("%.1f%%(%.2f/%.2f)", usedPercent, usedGB, totalGB)
	return &res
}

func GetCPUUsage() *string {
	totalPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(totalPercent) > 0 {
		res := fmt.Sprintf("%.2f%%", totalPercent[0])
		return &res
	}
	return nil
}

func GetDiskUsage() *string {
	usage, err := disk.Usage("/")
	if err != nil {
		return nil
	}
	res := fmt.Sprintf("%.1f%%(%d/%d)", usage.UsedPercent, usage.Used/(1<<30), usage.Total/(1<<30))
	return &res
}
