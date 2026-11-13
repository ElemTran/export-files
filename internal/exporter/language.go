package exporter

import (
	"path"
	"strings"
)

// languages 是扩展名到 Markdown 围栏语言标记的映射（键不含点，全小写）。
var languages = map[string]string{
	// 原 bash 版覆盖的扩展名
	"py":       "python",
	"js":       "javascript",
	"mjs":      "javascript",
	"cjs":      "javascript",
	"ts":       "typescript",
	"mts":      "typescript",
	"cts":      "typescript",
	"tsx":      "tsx",
	"jsx":      "jsx",
	"vue":      "vue",
	"java":     "java",
	"cpp":      "cpp",
	"cc":       "cpp",
	"cxx":      "cpp",
	"c++":      "cpp",
	"c":        "c",
	"h":        "cpp",
	"hpp":      "cpp",
	"hh":       "cpp",
	"hxx":      "cpp",
	"go":       "go",
	"rs":       "rust",
	"sh":       "bash",
	"bash":     "bash",
	"zsh":      "bash",
	"ksh":      "bash",
	"fish":     "fish",
	"md":       "markdown",
	"markdown": "markdown",
	"json":     "json",
	"jsonc":    "json",
	"json5":    "json",
	"yaml":     "yaml",
	"yml":      "yaml",
	"html":     "html",
	"htm":      "html",
	"css":      "css",
	"xml":      "xml",
	"sql":      "sql",
	"php":      "php",
	"rb":       "ruby",

	// 补充的扩展名
	"kt":      "kotlin",
	"kts":     "kotlin",
	"swift":   "swift",
	"cs":      "csharp",
	"dart":    "dart",
	"proto":   "protobuf",
	"graphql": "graphql",
	"gql":     "graphql",
	"toml":    "toml",
	"ini":     "ini",
	"cfg":     "ini",
	"conf":    "ini",
	"env":     "ini",
	"tf":      "hcl",
	"tfvars":  "hcl",
	"scss":    "scss",
	"sass":    "sass",
	"less":    "less",
	"ps1":     "powershell",
	"psm1":    "powershell",
	"psd1":    "powershell",
	"mk":      "makefile",
	"cmake":   "cmake",
	"bat":     "bat",
	"cmd":     "bat",
	"pl":      "perl",
	"pm":      "perl",
	"lua":     "lua",
	"r":       "r",
	"scala":   "scala",
	"groovy":  "groovy",
	"gradle":  "groovy",
	"ex":      "elixir",
	"exs":     "elixir",
	"erl":     "erlang",
	"hs":      "haskell",
	"clj":     "clojure",
	"jl":      "julia",
	"zig":     "zig",
	"nim":     "nim",
	"s":       "asm",
	"asm":     "asm",
	"vim":     "vim",
}

// namedLanguages 是按完整文件名（小写）判定的语言标记。
var namedLanguages = map[string]string{
	"dockerfile":          "dockerfile",
	"containerfile":       "dockerfile",
	"makefile":            "makefile",
	"gnumakefile":         "makefile",
	"bsdmakefile":         "makefile",
	"cmakelists.txt":      "cmake",
	"docker-compose.yml":  "yaml",
	"docker-compose.yaml": "yaml",
}

// Language 返回文件对应的 Markdown 围栏语言标记；无法识别时返回空串。
func Language(name string) string {
	base := strings.ToLower(path.Base(strings.ReplaceAll(name, `\`, "/")))
	if lang, ok := namedLanguages[base]; ok {
		return lang
	}
	// Dockerfile.dev、Makefile.am、app.dockerfile 这类变体
	if strings.HasPrefix(base, "dockerfile.") || strings.HasSuffix(base, ".dockerfile") {
		return "dockerfile"
	}
	if strings.HasPrefix(base, "makefile.") {
		return "makefile"
	}
	// docker-compose.*.yml、compose*.yml 仍然按 YAML 输出
	if strings.HasPrefix(base, "docker-compose") || strings.HasPrefix(base, "compose") {
		if ext := path.Ext(base); ext == ".yml" || ext == ".yaml" {
			return "yaml"
		}
	}
	// 无扩展名时留空；点开头的 .env 由 path.Ext 返回 ".env"，按扩展名映射
	ext := path.Ext(base)
	if ext == "" {
		return ""
	}
	return languages[ext[1:]]
}
