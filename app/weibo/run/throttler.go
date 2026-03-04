package run

import "time"

type ErrorThrottler struct {
	threshold  int           // 阈值
	window     time.Duration // 时间窗口
	cooldown   time.Duration // 冷却时间
	timestamps []time.Time   // 窗口内的错误时间戳
	lastPrint  time.Time     // 上次打印时间
}

func NewErrorThrottler(threshold int, window time.Duration, cooldown time.Duration) *ErrorThrottler {
	return &ErrorThrottler{
		threshold: threshold,
		window:    window,
		cooldown:  cooldown,
	}
}

// RecordError 记录错误，返回 true 表示需要打印日志
func (et *ErrorThrottler) RecordError(now time.Time) bool {
	// 清理窗口外的时间戳
	cutoff := now.Add(-et.window)
	i := 0
	for ; i < len(et.timestamps); i++ {
		if et.timestamps[i].After(cutoff) {
			break
		}
	}
	et.timestamps = et.timestamps[i:]

	// 添加当前错误
	et.timestamps = append(et.timestamps, now)

	// 检查是否达到阈值并满足冷却条件
	if len(et.timestamps) >= et.threshold {
		if et.lastPrint.IsZero() || now.Sub(et.lastPrint) > et.cooldown {
			et.lastPrint = now
			return true
		}
	}
	return false
}
