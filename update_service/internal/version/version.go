// Package version 处理 Shadow Worker 版本号比较。
//
// 版本号格式：YYYY.MM.DD.NN，例如 2026.07.02.02。
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version 是解析后的版本号。
type Version struct {
	Year  int
	Month int
	Day   int
	Seq   int
	Raw   string
}

// Parse 解析版本字符串。
func Parse(s string) (Version, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) != 4 {
		return Version{}, fmt.Errorf("invalid version %q, expected YYYY.MM.DD.NN", s)
	}
	nums := make([]int, 4)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, fmt.Errorf("invalid version %q: %w", s, err)
		}
		nums[i] = n
	}
	return Version{
		Year:  nums[0],
		Month: nums[1],
		Day:   nums[2],
		Seq:   nums[3],
		Raw:   s,
	}, nil
}

// Less 报告 v 是否小于 other（按年/月/日/序号逐位比较）。
func (v Version) Less(other Version) bool {
	if v.Year != other.Year {
		return v.Year < other.Year
	}
	if v.Month != other.Month {
		return v.Month < other.Month
	}
	if v.Day != other.Day {
		return v.Day < other.Day
	}
	return v.Seq < other.Seq
}

// Compare 返回 -1/0/1。
func (v Version) Compare(other Version) int {
	if v.Raw == other.Raw {
		return 0
	}
	if v.Less(other) {
		return -1
	}
	return 1
}
