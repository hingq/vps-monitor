// Package humanize 提供把原始数值转换为人类可读字符串的辅助函数。
package humanize

import "fmt"

// FormatBytes 将字节数格式化为可读单位（B/KB/MB/GB/TB）。
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGT"[exp])
}
