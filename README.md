# export-files

[![CI](https://github.com/ElemTran/export-files/actions/workflows/ci.yml/badge.svg)](https://github.com/ElemTran/export-files/actions/workflows/ci.yml)
[![Release](https://github.com/ElemTran/export-files/actions/workflows/release.yml/badge.svg)](https://github.com/ElemTran/export-files/actions/workflows/release.yml)

把目录 / 文件内容导出成 Markdown，方便贴给 LLM 阅读。Go 单模块命令行工具，
用法与输出格式兼容原 bash 版本，并修掉了原脚本的文件名、二进制判定与忽略规则缺陷。

## 用法

```
export-files [选项] <路径1> [路径2] ...
```

路径可以是目录或单个文件，可混合传入；目录会递归遍历。每个文件输出为一段：

````markdown
## <相对路径>
```<语言标记>
<文件内容>
```
````

- 相对路径统一用 `/` 分隔（Windows 上同样输出 `/`），不带 `./` 前缀。
  传入 `src` 时输出 `src/foo.go`；传入单个文件 `src/App.vue` 时输出 `src/App.vue`。
- 文件内容不以换行结尾时会补一个 `\n`，避免正文与闭合围栏粘连。
- 输出按展示路径排序，编码 UTF-8 无 BOM，换行统一为 LF。

```console
$ export-files src > output.md
$ export-files . --ignore 'dist/*' --max-size 1M | head -40
$ export-files src tests docs -o output.md
```

## 选项

长短两种写法都接受（也支持 `--opt=value` 形式），选项与路径可以交错出现。

| 选项 | 说明 |
| --- | --- |
| `-h`, `--help` | 显示帮助 |
| `-o`, `--output FILE` | 写入文件而不是 stdout（UTF-8 无 BOM，LF） |
| `--max-size N` | 跳过大于 N 字节的文件；支持 `1024` / `10K` / `5M` / `1G`（1024 进制） |
| `--ignore GLOB` | 追加忽略规则，可重复 |
| `--no-ignore` | 清空默认忽略规则（`--ignore` 仍生效） |
| `--include-hidden` | 包含以 `.` 开头的文件与目录（默认跳过） |
| `-L`, `--follow` | 跟随符号链接（默认跳过并给警告） |

退出码：`0` 成功；`1` 参数错误、未给出路径，或存在不存在的路径（此时仍处理其余路径，
并在 stderr 逐条警告）。

`--` 之后的参数一律视为路径，可用来传入以 `-` 开头的文件名。

### 输出到文件

`-o` 会先写同目录下的临时文件（`.export-files-*`），成功后再 `rename` 到目标；
目标文件与临时文件都会从遍历结果中排除，因此 `export-files . -o out.md` 不会把
`out.md` 自己扫进去，重复执行结果幂等。

### 忽略规则

默认忽略版本控制与依赖目录（`.git`、`node_modules`、`__pycache__`、`.venv`、`vendor` 等）、
构建缓存，以及常见二进制（`*.pyc`、`*.o`、`*.so`、`*.exe`、`*.dll`、图片、字体、压缩包、
音视频、`*.db`/`*.sqlite3` 等）。

匹配使用 `path.Match` 语义，但对相对路径的**每一段后缀**都做一次匹配，因此：

- `*.pyc` 等价于在任意深度按 basename 匹配；
- `*/node_modules/*` 能命中 `node_modules/pkg/index.js` 与 `a/node_modules/pkg/index.js`；
- `dist/*` 同时命中 `dist/a.js` 与 `other/dist/a.js`；
- 以 `/` 结尾的规则（如 `build/`）按目录自身匹配。

## 与 bash 原版的差异

| 方面 | bash 原版 | 本工具 |
| --- | --- | --- |
| 遍历 | `find ... \| sort`，配合 `IFS= read -r`，文件名含换行时仍会错行 | 直接读目录项，任意空格/换行/Unicode 文件名都正确 |
| 二进制判定 | 依赖外部 `file` 命令 | 读前 8KB，含 NUL 即判定为二进制，无外部依赖 |
| 文本末尾无换行 | 正文与 ` ``` ` 粘连 | 自动补一个换行 |
| 隐藏文件 | 用正则 `/\.` 判断，会误伤任何含 `/.` 的路径 | 只看每一段是否以 `.` 开头，可用 `--include-hidden` 打开 |
| 忽略规则 | 硬编码 `node_modules`/`__pycache__`/`.git` | 默认规则表 + `--ignore` 追加 + `--no-ignore` 清空 |
| 语言标记 | 约 24 种扩展名 | 覆盖原有全部扩展名，并补充 kotlin/swift/csharp/dart/protobuf/graphql/toml/ini/hcl/scss/sass/less/powershell/makefile/cmake 等；`Dockerfile`、`Makefile`、`CMakeLists.txt`、`*.mk`、`docker-compose*.yml` 按文件名识别 |
| 选项 | 仅 `-h`/`--help` | 见上表（大小限制、忽略规则、隐藏文件、符号链接、输出文件） |
| 输出文件 | 只能 shell 重定向 | `-o` 原子写入并自动排除自身 |
| 路径不存在 | 警告后继续，退出码仍为 0 | 警告后继续，最终退出码为 1 |
| 符号链接 | 跟随 `find -type f` 的表现（实际不跟随，静默跳过） | 默认跳过并警告，`-L` 跟随且检测链接环 |
| 换行 | 依赖环境 | 始终输出 LF，Windows 上也不产生 CRLF |

保持不变的行为：段格式（`## 路径` + 围栏 + 空行）、二进制占位文案 `[二进制文件，已跳过]`、
无扩展名文件语言标记留空、多个路径按参数顺序处理。

## 构建

需要 Go 1.27+。

```console
# 本机
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/export-files-linux-amd64 ./cmd/export-files

# 交叉编译 Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" \
  -o dist/export-files-windows-amd64.exe ./cmd/export-files

# 或直接用 Makefile（同时产出两个平台）
make build
```

产物位于 `dist/`（已在 `.gitignore` 中忽略）。

## 开发与门禁

门禁配置与 `star-entry-gateway/cloudflare` 一致：`.golangci.yml`（golangci-lint v2）
加 Makefile 目标。

```console
make fmt-check     # gofmt 检查
make fix-check     # go fix + modernize 检查（无未提交改动）
make tidy-check    # go mod tidy 检查
make lint          # golangci-lint
make test          # go test -race
make deadcode      # 死代码扫描
make vuln          # govulncheck
make check         # 以上全量
make pre-commit    # == make check
```

开发工具（deadcode / modernize / govulncheck）通过 `go.mod` 的 `tool` 指令管理，
用 `go tool <name>` 调用；golangci-lint 用 `make install-lint` 安装。

## 代码结构

```
cmd/export-files/
├── main.go         CLI 装配：参数解析、stdout/文件输出、退出码（唯一允许 os.Exit 的地方）
└── size.go         --max-size 的 1024 / 10K / 5M / 1G 解析
internal/exporter/
├── exporter.go     导出核心：遍历、隐藏/忽略/排除判定、二进制嗅探、围栏输出、符号链接
├── ignore.go       默认忽略规则表 + glob 匹配（后缀候选匹配）
└── language.go     扩展名/文件名到围栏语言标记的映射
```

模块名 `github.com/ElemTran/export-files`（单模块，无第三方运行时依赖）。

## 发布

推送 `v*` 标签即触发 Release 工作流，构建 Linux / macOS / Windows 的 amd64 与 arm64 产物，
每个平台同时提供**归档包**（`tar.gz` / `zip`，内含二进制 + README，Unix 归档可复现）与
**裸二进制**（下载即可运行），另有 `sha256sums.txt`，并创建 GitHub Release：

```console
git tag v0.1.0 && git push origin v0.1.0
```

CI（`.github/workflows/ci.yml`）在每次 push / PR 时跑 `gofmt`、`go vet`、`golangci-lint`、
`go test -race`、`deadcode` 与三平台编译检查。
