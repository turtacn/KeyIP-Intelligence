// Phase 12 - File #287: cmd/keyip/main.go
// 生成计划:
// ---
// 继续输出 287 `cmd/keyip/main.go` 要实现 CLI 客户端入口程序。
//
// 实现要求:
//
// * **功能定位**：KeyIP-Intelligence 命令行客户端入口，提供本地交互式操作能力，支持专利检索、分子评估、侵权分析、生命周期管理、报告生成等命令
// * **核心实现**：
//   * 定义 main 函数：初始化 CLI 根命令
//   * 实现配置加载：支持 --config 指定配置文件、--server 指定 API 服务器地址
//   * 实现 API 客户端初始化：基于配置创建 pkg/client.Client 实例
//   * 注册子命令：search、assess、lifecycle、report（来自 internal/interfaces/cli/）
//   * 实现全局 Flag：--config、--server、--token、--output-format（json/table/yaml）、--verbose
//   * 实现版本命令：keyip version 输出版本号、构建时间、Git commit
//   * 实现补全命令：keyip completion bash/zsh/fish
//   * 实现错误处理：统一捕获子命令错误，格式化输出到 stderr
// * **业务逻辑**：
//   * CLI 通过 HTTP/gRPC 客户端与 API 服务器通信，不直接访问数据库
//   * 支持 KEYIP_SERVER 和 KEYIP_TOKEN 环境变量
//   * 输出格式默认 table，支持 json/yaml 用于脚本集成
//   * --verbose 开启详细日志输出
// * **依赖关系**：
//   * 依赖：internal/interfaces/cli/*、pkg/client/*、internal/config
//   * 被依赖：Makefile、用户直接使用
// * **测试要求**：入口程序不做单元测试，通过 E2E 测试覆盖
// * **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
// ---
package main

import (
	"fmt"
	"os"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/cli"
	"github.com/turtacn/KeyIP-Intelligence/pkg/client"
)

// Build-time variables injected via ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Create root command with build info
	buildInfo := &cli.BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	}

	rootCmd := cli.NewRootCommand(buildInfo)

	// Add persistent pre-run to initialize client
	rootCmd.PersistentPreRunE = func(cmd *cli.Command, args []string) error {
		// Skip client init for version and completion commands
		if cmd.Name() == "version" || cmd.Name() == "completion" {
			return nil
		}

		// Resolve configuration
		configPath := cmd.FlagString("config")
		serverAddr := cmd.FlagString("server")
		token := cmd.FlagString("token")

		// Environment variable fallbacks
		if serverAddr == "" {
			serverAddr = os.Getenv("KEYIP_SERVER")
		}
		if token == "" {
			token = os.Getenv("KEYIP_TOKEN")
		}

		// Load config if path provided
		var cfg *config.Config
		if configPath != "" {
			var err error
			cfg, err = config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		// Determine server address
		if serverAddr == "" && cfg != nil {
			serverAddr = cfg.Client.ServerAddress
		}
		if serverAddr == "" {
			serverAddr = "http://localhost:8080"
		}

		// Create API client
		opts := []client.Option{
			client.WithBaseURL(serverAddr),
		}
		if token != "" {
			opts = append(opts, client.WithAuthToken(token))
		}
		if cfg != nil && cfg.Client.Timeout > 0 {
			opts = append(opts, client.WithTimeout(cfg.Client.Timeout))
		}

		apiClient, err := client.New(opts...)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Store client in command context
		cmd.SetClient(apiClient)
		return nil
	}

	// Register global flags
	rootCmd.AddPersistentFlag("config", "c", "", "path to configuration file")
	rootCmd.AddPersistentFlag("server", "s", "", "API server address (e.g., http://localhost:8080)")
	rootCmd.AddPersistentFlag("token", "t", "", "authentication token")
	rootCmd.AddPersistentFlag("output", "o", "table", "output format: table, json, yaml")
	rootCmd.AddPersistentFlag("verbose", "v", "false", "enable verbose output")

	// Register sub-commands
	rootCmd.AddCommand(cli.NewSearchCommand())
	rootCmd.AddCommand(cli.NewAssessCommand())
	rootCmd.AddCommand(cli.NewLifecycleCommand())
	rootCmd.AddCommand(cli.NewReportCommand())
	rootCmd.AddCommand(newVersionCommand(buildInfo))
	rootCmd.AddCommand(newCompletionCommand())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// newVersionCommand creates the version sub-command.
func newVersionCommand(info *cli.BuildInfo) *cli.Command {
	return &cli.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cli.Command, args []string) error {
			format := cmd.FlagString("output")
			switch format {
			case "json":
				fmt.Printf(`{"version":"%s","commit":"%s","build_date":"%s"}`+"\n",
					info.Version, info.Commit, info.BuildDate)
			default:
				fmt.Printf("KeyIP-Intelligence CLI\n")
				fmt.Printf("  Version:    %s\n", info.Version)
				fmt.Printf("  Commit:     %s\n", info.Commit)
				fmt.Printf("  Build Date: %s\n", info.BuildDate)
			}
			return nil
		},
	}
}

// newCompletionCommand creates the shell completion sub-command.
func newCompletionCommand() *cli.Command {
	return &cli.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cli.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", args[0])
			}
		},
	}
}

//Personal.AI order the ending
