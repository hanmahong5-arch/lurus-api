package common

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/shirou/gopsutil/cpu"
)

// Monitor 定时监控cpu使用率，超过阈值输出pprof文件
func Monitor() {
	for {
		percent, err := cpu.Percent(time.Second, false)
		if err != nil {
			panic(err)
		}
		if percent[0] > 80 {
			fmt.Println("cpu usage too high")
			// write pprof file
			if _, err := os.Stat("./pprof"); os.IsNotExist(err) {
				err := os.Mkdir("./pprof", os.ModePerm)
				if err != nil {
					SysLog("创建pprof文件夹失败 " + err.Error())
					continue
				}
			}
			f, err := os.Create("./pprof/" + fmt.Sprintf("cpu-%s.pprof", time.Now().Format("20060102150405")))
			if err != nil {
				SysLog("创建pprof文件失败 " + err.Error())
				continue
			}
			err = pprof.StartCPUProfile(f)
			if err != nil {
				SysLog("启动pprof失败 " + err.Error())
				continue
			}
			time.Sleep(10 * time.Second) // profile for 30 seconds
			pprof.StopCPUProfile()
			f.Close()
		}
		time.Sleep(30 * time.Second)
	}
}

// MonitorWithContext monitors CPU usage with context cancellation support.
func MonitorWithContext(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			SysLog("CPU monitor stopped")
			return
		case <-ticker.C:
			percent, err := cpu.Percent(time.Second, false)
			if err != nil {
				SysError("CPU percent error: " + err.Error())
				continue
			}
			if percent[0] > 80 {
				fmt.Println("cpu usage too high")
				if _, err := os.Stat("./pprof"); os.IsNotExist(err) {
					if err := os.Mkdir("./pprof", os.ModePerm); err != nil {
						SysLog("创建pprof文件夹失败 " + err.Error())
						continue
					}
				}
				f, err := os.Create("./pprof/" + fmt.Sprintf("cpu-%s.pprof", time.Now().Format("20060102150405")))
				if err != nil {
					SysLog("创建pprof文件失败 " + err.Error())
					continue
				}
				if err := pprof.StartCPUProfile(f); err != nil {
					SysLog("启动pprof失败 " + err.Error())
					f.Close()
					continue
				}
				// Profile for 10 seconds, checking for context cancellation
				select {
				case <-ctx.Done():
					pprof.StopCPUProfile()
					f.Close()
					SysLog("CPU monitor stopped during profiling")
					return
				case <-time.After(10 * time.Second):
					pprof.StopCPUProfile()
					f.Close()
				}
			}
		}
	}
}
