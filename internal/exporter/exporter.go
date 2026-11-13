// Package exporter 把目录或文件内容导出为 Markdown。
//
// 每个文件输出为一段：
//
//	## <相对路径>
//	```<语言标记>
//	<文件内容>
//	```
//
// 输出统一使用 LF 换行与 UTF-8 编码。
package exporter

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// binarySniffLen 是判断二进制文件时嗅探的字节数。
const binarySniffLen = 8192

// binaryPlaceholder 是二进制文件被跳过时代入正文的占位内容。
const binaryPlaceholder = "[二进制文件，已跳过]"

// Options 控制一次导出的行为。零值表示：不限制大小、使用默认忽略规则、
// 跳过隐藏文件、跳过符号链接。
type Options struct {
	// MaxSize 大于 0 时跳过体积超过该字节数的文件。
	MaxSize int64
	// Ignore 是追加到默认规则之后的忽略 glob。
	Ignore []string
	// NoIgnore 为 true 时不使用默认忽略规则（Ignore 仍生效）。
	NoIgnore bool
	// IncludeHidden 为 true 时处理以 . 开头的文件与目录。
	IncludeHidden bool
	// Follow 为 true 时跟随符号链接。
	Follow bool
	// Exclude 是需要跳过的绝对路径，用于排除输出文件自身。
	Exclude []string
	// Warnf 接收警告（默认丢弃），调用方通常接到 stderr。
	Warnf func(format string, args ...any)
}

// PathsNotFoundError 表示有输入路径不存在；其余路径仍会被正常导出。
type PathsNotFoundError struct {
	Paths []string
}

func (e *PathsNotFoundError) Error() string {
	return "路径不存在: " + strings.Join(e.Paths, ", ")
}

// entry 是待输出的一个文件。
type entry struct {
	display string // 输出到 ## 后的路径，使用 / 分隔
	abs     string // 读取内容用的绝对路径
}

// Exporter 承载一次导出所需的配置与状态。
type Exporter struct {
	opts    Options
	out     *bufio.Writer
	ignores []string
	exclude map[string]bool
	visited map[string]bool
	files   []entry
	missing []string
	warnf   func(format string, args ...any)
}

// New 创建 Exporter，内容写到 w。
func New(w io.Writer, opts Options) *Exporter {
	e := &Exporter{
		opts:    opts,
		out:     bufio.NewWriter(w),
		exclude: make(map[string]bool),
		visited: make(map[string]bool),
		warnf:   opts.Warnf,
	}
	if e.warnf == nil {
		e.warnf = func(string, ...any) {}
	}
	if !opts.NoIgnore {
		e.ignores = append(e.ignores, defaultIgnores...)
	}
	e.ignores = append(e.ignores, opts.Ignore...)
	for _, p := range opts.Exclude {
		if p == "" {
			continue
		}
		if abs, err := filepath.Abs(p); err == nil {
			e.exclude[filepath.Clean(abs)] = true
		}
	}
	return e
}

// Run 依次扫描 paths 并输出。全部成功时返回 nil；若有输入路径不存在
// 则返回 *PathsNotFoundError，已存在的路径仍会被导出。
func (e *Exporter) Run(paths []string) error {
	for _, p := range paths {
		e.collectRoot(p)
	}
	slices.SortStableFunc(e.files, func(a, b entry) int {
		return cmp.Compare(a.display, b.display)
	})
	for _, f := range e.files {
		e.emit(f)
	}
	if err := e.out.Flush(); err != nil {
		return fmt.Errorf("写入输出失败: %w", err)
	}
	if len(e.missing) > 0 {
		return &PathsNotFoundError{Paths: e.missing}
	}
	return nil
}

// collectRoot 处理一个命令行输入参数（目录或文件）。
func (e *Exporter) collectRoot(given string) {
	info, err := os.Stat(given)
	if err != nil {
		e.miss(given)
		return
	}
	abs, err := filepath.Abs(given)
	if err != nil {
		e.miss(given)
		return
	}
	display := cleanDisplay(given)
	switch {
	case info.IsDir():
		if e.isExcluded(abs) {
			return
		}
		e.visited[e.resolve(abs)] = true
		e.collectDir(abs, "", display)
	case info.Mode().IsRegular():
		e.files = append(e.files, entry{display: display, abs: abs})
	default:
		e.warnf("警告: 跳过非普通文件: %s", given)
	}
}

// collectDir 递归收集目录中的文件。rel 是相对该根路径的 / 分隔路径，
// displayDir 是对外展示的路径前缀。
func (e *Exporter) collectDir(absDir, rel, displayDir string) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		e.warnf("警告: 无法读取目录 %s: %v", displayDir, err)
		return
	}
	for _, de := range entries {
		name := de.Name()
		if !e.opts.IncludeHidden && strings.HasPrefix(name, ".") {
			continue
		}
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		if matchIgnores(e.ignores, childRel) {
			continue
		}
		childAbs := filepath.Join(absDir, name)
		if e.isExcluded(childAbs) {
			continue
		}
		childDisplay := name
		if displayDir != "" {
			childDisplay = displayDir + "/" + name
		}
		e.collectEntry(de, childAbs, childRel, childDisplay)
	}
}

// collectEntry 按类型处理目录项：目录递归、符号链接按配置处理、普通文件入队。
func (e *Exporter) collectEntry(de os.DirEntry, abs, rel, display string) {
	switch {
	case de.Type()&os.ModeSymlink != 0:
		e.collectSymlink(abs, rel, display)
	case de.IsDir():
		e.collectDir(abs, rel, display)
	case de.Type().IsRegular():
		e.files = append(e.files, entry{display: display, abs: abs})
	}
}

// collectSymlink 处理符号链接：默认只给警告并跳过，Follow 时解析目标。
func (e *Exporter) collectSymlink(abs, rel, display string) {
	if !e.opts.Follow {
		e.warnf("警告: 跳过符号链接: %s", display)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		e.warnf("警告: 符号链接无法解析: %s: %v", display, err)
		return
	}
	switch {
	case info.IsDir():
		real := e.resolve(abs)
		if e.visited[real] {
			e.warnf("警告: 跳过符号链接环: %s", display)
			return
		}
		e.visited[real] = true
		e.collectDir(abs, rel, display)
	case info.Mode().IsRegular():
		e.files = append(e.files, entry{display: display, abs: abs})
	default:
		e.warnf("警告: 跳过非普通文件: %s", display)
	}
}

// emit 把单个文件写入输出。
func (e *Exporter) emit(f entry) {
	info, err := os.Stat(f.abs)
	if err != nil {
		e.warnf("警告: 无法读取文件 %s: %v", f.display, err)
		return
	}
	if !info.Mode().IsRegular() {
		return
	}
	if e.opts.MaxSize > 0 && info.Size() > e.opts.MaxSize {
		return
	}
	data, err := os.ReadFile(f.abs)
	if err != nil {
		e.warnf("警告: 无法读取文件 %s: %v", f.display, err)
		return
	}
	if isBinary(data) {
		e.writeBlock(f.display, "", []byte(binaryPlaceholder+"\n"))
		return
	}
	e.writeBlock(f.display, Language(f.display), data)
}

// writeBlock 按统一格式写出一段：标题、围栏、正文、闭合围栏、空行。
func (e *Exporter) writeBlock(display, lang string, data []byte) {
	fmt.Fprintf(e.out, "## %s\n```%s\n", display, lang)
	e.out.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		e.out.WriteByte('\n')
	}
	e.out.WriteString("```\n\n")
}

func (e *Exporter) isExcluded(abs string) bool {
	if len(e.exclude) == 0 {
		return false
	}
	return e.exclude[filepath.Clean(abs)]
}

// resolve 返回路径解析符号链接后的形式，用于检测链接环。
func (e *Exporter) resolve(abs string) string {
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(real)
}

func (e *Exporter) miss(given string) {
	e.missing = append(e.missing, given)
	e.warnf("警告: 路径不存在: %s", given)
}

// isBinary 用前 8KB 是否含 NUL 字节判断二进制文件。
func isBinary(data []byte) bool {
	n := min(len(data), binarySniffLen)
	return slices.Contains(data[:n], byte(0))
}

// cleanDisplay 把命令行给出的路径转成展示用形式：统一 / 分隔、
// 去掉多余的 ./ 与尾随分隔符；"." 变为空串（表示当前目录）。
func cleanDisplay(p string) string {
	d := filepath.ToSlash(filepath.Clean(p))
	if d == "." {
		return ""
	}
	return d
}
