package main

import (
	"errors"
	"strconv"
	"strings"
)

// sizeSuffixes 是 --max-size 支持的后缀，按最长优先匹配。
var sizeSuffixes = []struct {
	suffix string
	mult   int64
}{
	{"TB", 1 << 40},
	{"GB", 1 << 30},
	{"MB", 1 << 20},
	{"KB", 1 << 10},
	{"T", 1 << 40},
	{"G", 1 << 30},
	{"M", 1 << 20},
	{"K", 1 << 10},
	{"B", 1},
}

// parseSize 解析文件大小：纯数字为字节，后缀 K/M/G/T 与 KB/MB/GB/TB 为 1024 进制。
func parseSize(s string) (int64, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	if t == "" {
		return 0, errors.New("空值")
	}
	mult := int64(1)
	for _, sfx := range sizeSuffixes {
		if strings.HasSuffix(t, sfx.suffix) {
			mult = sfx.mult
			t = strings.TrimSpace(strings.TrimSuffix(t, sfx.suffix))
			break
		}
	}
	if t == "" {
		return 0, errors.New("缺少数字")
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, errors.New("不能为负数")
	}
	return n * mult, nil
}
