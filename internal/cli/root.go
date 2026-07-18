// Package cli 定义 stockagent 的命令行界面。
package cli

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version 由 goreleaser 通过 ldflags 注入；
// 未注入时 (如 go install @vX.Y.Z) 回退到 Go 模块版本。
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok &&
		bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		Version = bi.Main.Version
	}
}

var cfgFile string

// NewRootCmd 构建根命令。
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "stockagent",
		Short: "A 股 AI 投研多智能体 CLI",
		Long: `stockagent 用多个 AI 分析师智能体并行分析 A 股个股，输出中文投研报告。

仅供学习与技术研究，不构成任何投资建议。`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认 ./stockagent.yaml 或 ~/.stockagent.yaml)")
	root.AddCommand(newAnalyzeCmd(), newVersionCmd())
	return root
}

// Execute 运行根命令，失败时打印错误并以非零码退出。
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}
