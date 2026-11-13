// Command export-files 把目录或文件内容导出为 Markdown，便于贴给 LLM 阅读。
//
// 该包只负责命令行参数解析、输出目标装配与退出码；遍历与格式化逻辑在
// github.com/ElemTran/export-files/internal/exporter 中，便于单独测试。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ElemTran/export-files/internal/exporter"
)

const usageText = `用法: export-files [选项] <路径1> [路径2] ...

把目录或文件的内容导出为 Markdown：每个文件一段
  ## <相对路径>
  ` + "```" + `<语言标记>
  <文件内容>
  ` + "```" + `

目录会递归遍历，路径可以是目录与文件的混合。

选项:
  -h, --help            显示此帮助信息
  -o, --output FILE     写入文件而不是 stdout（UTF-8 无 BOM，LF 换行）
      --max-size N      跳过大于 N 字节的文件；支持 1024 / 10K / 5M / 1G
      --ignore GLOB     追加忽略规则，可重复
      --no-ignore       清空默认忽略规则
      --include-hidden  包含以 . 开头的文件与目录（默认跳过）
  -L, --follow          跟随符号链接（默认跳过并给警告）

示例:
  export-files src > output.md
  export-files src tests docs -o output.md
  export-files . --ignore 'dist/*' --max-size 1M | head -40
`

// config 是解析后的命令行配置。
type config struct {
	output        string
	maxSize       int64
	ignores       []string
	noIgnore      bool
	includeHidden bool
	follow        bool
	help          bool
	paths         []string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run 执行一次完整流程并返回退出码：0 成功，1 参数错误或存在缺失路径。
func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n\n%s", err, usageText)
		return 1
	}
	if cfg.help {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	if len(cfg.paths) == 0 {
		fmt.Fprintf(stderr, "错误: 请指定至少一个路径（目录或文件）\n\n%s", usageText)
		return 1
	}
	warn := func(format string, a ...any) { fmt.Fprintf(stderr, format+"\n", a...) }
	opts := cfg.options(warn)
	if cfg.output == "" {
		return reportRunError(exporter.New(stdout, opts).Run(cfg.paths), warn)
	}
	return writeToFile(cfg.output, opts, cfg.paths, warn)
}

func (c *config) options(warn func(string, ...any)) exporter.Options {
	return exporter.Options{
		MaxSize:       c.maxSize,
		Ignore:        c.ignores,
		NoIgnore:      c.noIgnore,
		IncludeHidden: c.includeHidden,
		Follow:        c.follow,
		Warnf:         warn,
	}
}

// writeToFile 先把结果写到目标目录下的临时文件，再原子地 rename 到目标，
// 并把目标与临时文件都排除在扫描范围之外。
func writeToFile(output string, opts exporter.Options, paths []string, warn func(string, ...any)) int {
	target, err := filepath.Abs(output)
	if err != nil {
		warn("错误: 无法解析输出路径 %s: %v", output, err)
		return 1
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		warn("错误: 无法创建输出目录 %s: %v", dir, err)
		return 1
	}
	tmp, err := os.CreateTemp(dir, ".export-files-*")
	if err != nil {
		warn("错误: 无法创建临时文件: %v", err)
		return 1
	}
	tmpName := tmp.Name()
	opts.Exclude = slices.Concat(opts.Exclude, []string{target, tmpName})
	runErr := exporter.New(tmp, opts).Run(paths)
	if err := finishTemp(tmp, runErr); err != nil {
		warn("错误: %v", err)
		os.Remove(tmpName)
		return 1
	}
	if err := os.Rename(tmpName, target); err != nil {
		warn("错误: 无法写入 %s: %v", target, err)
		os.Remove(tmpName)
		return 1
	}
	return reportRunError(runErr, warn)
}

// finishTemp 关闭临时文件并修正权限；仅在错误不是「路径不存在」时返回错误。
func finishTemp(tmp *os.File, runErr error) error {
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if runErr != nil && !isPathsNotFound(runErr) {
		return runErr
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("设置输出文件权限失败: %w", err)
	}
	return nil
}

// reportRunError 打印错误并返回退出码；路径缺失的警告已在导出时逐条打印。
func reportRunError(err error, warn func(string, ...any)) int {
	if err == nil {
		return 0
	}
	if !isPathsNotFound(err) {
		warn("错误: %v", err)
	}
	return 1
}

func isPathsNotFound(err error) bool {
	var notFound *exporter.PathsNotFoundError
	return errors.As(err, &notFound)
}

// parseArgs 解析命令行，支持选项与路径交错出现以及 -- 终止符，
// 并同时接受 -o/--output 这类长短写法（也支持 --output=FILE）。
func parseArgs(args []string) (*config, error) {
	cfg := &config{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.paths = append(cfg.paths, args[i+1:]...)
			return cfg, nil
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			cfg.paths = append(cfg.paths, arg)
			continue
		}
		consumed, err := cfg.applyFlag(arg, args[i+1:])
		if err != nil {
			return nil, err
		}
		i += consumed
	}
	return cfg, nil
}

// applyFlag 应用一个选项 arg，rest 是其后的参数；返回额外消费的参数个数。
func (c *config) applyFlag(arg string, rest []string) (int, error) {
	name, value, hasValue := splitFlag(arg)
	consumed := 0
	valueOf := func() (string, error) {
		if hasValue {
			return value, nil
		}
		if len(rest) == 0 {
			return "", fmt.Errorf("选项 %s 需要一个值", arg)
		}
		consumed = 1
		return rest[0], nil
	}
	switch name {
	case "h", "help":
		c.help = true
	case "L", "follow":
		c.follow = true
	case "no-ignore":
		c.noIgnore = true
	case "include-hidden":
		c.includeHidden = true
	case "o", "output", "max-size", "ignore":
		v, err := valueOf()
		if err != nil {
			return 0, err
		}
		if err := c.setValue(name, v); err != nil {
			return 0, fmt.Errorf("选项 %s: %w", arg, err)
		}
		return consumed, nil
	default:
		return 0, fmt.Errorf("未知选项: %s", arg)
	}
	if hasValue {
		return 0, fmt.Errorf("选项 %s 不接受值", arg)
	}
	return 0, nil
}

// setValue 写入需要取值的选项。
func (c *config) setValue(name, v string) error {
	switch name {
	case "o", "output":
		if v == "" {
			return errors.New("值不能为空")
		}
		c.output = v
	case "max-size":
		size, err := parseSize(v)
		if err != nil {
			return fmt.Errorf("值无效 %q（示例: 1024、10K、5M、1G）", v)
		}
		c.maxSize = size
	case "ignore":
		if strings.TrimSpace(v) == "" {
			return errors.New("glob 不能为空")
		}
		c.ignores = append(c.ignores, v)
	}
	return nil
}

// splitFlag 拆出选项名与 = 后的值，去掉全部前导 -。
func splitFlag(arg string) (name, value string, hasValue bool) {
	s := strings.TrimLeft(arg, "-")
	if name, value, ok := strings.Cut(s, "="); ok {
		return name, value, true
	}
	return s, "", false
}
