// Phase 11 - File 248 of 349
// 生成计划:
// ---
// 继续输出 248 `internal/interfaces/cli/root.go` 要实现 CLI 根命令定义与全局初始化。
//
// 实现要求:
// * 功能定位：CLI 应用的入口根命令，负责全局 Flag 注册、配置加载、日志初始化、子命令挂载
// * 核心实现：
//   - RootOptions 结构体（ConfigPath/LogLevel/OutputFormat/Verbose/NoColor/Timeout/ServerAddr）
//   - CLIContext 结构体（Config/Logger/Client/OutputFormat/Verbose/NoColor）
//   - NewRootCommand() 创建根命令并注册 PersistentFlags 和子命令
//   - initConfig/initLogger/initClient 初始化链
//   - GetCLIContext 从 cobra.Command.Context 提取上下文
//   - Execute() 入口函数
//   - PrintResult/PrintError/PrintSuccess/FormatTable 输出辅助
// * 业务逻辑：配置搜索顺序、默认值、版本注入、输出格式化
// * 依赖：cobra、internal/config、logging、pkg/client
// * 被依赖：cmd/keyip/main.go、所有 CLI 子命令
// * 测试要求：Flag 解析、配置优先级、CLIContext 构建、输出格式化
// * 强制约束：文件最后一行必须为 `//Personal.AI order the ending`
// ---
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/client"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Build-time variables injected via ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// BuildInfo holds version information injected at build time.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// Command is an alias for cobra.Command for backward compatibility.
type Command = cobra.Command

// cliContextKey is the context key for CLIContext.
type cliContextKey struct{}

// RootOptions holds global CLI flags.
type RootOptions struct {
	ConfigPath   string
	LogLevel     string
	OutputFormat string
	Verbose      bool
	NoColor      bool
	Timeout      time.Duration
	ServerAddr   string
}

// CLIContext carries initialized dependencies through the command tree.
type CLIContext struct {
	Config       *config.Config
	Logger       logging.Logger
	Client       *client.Client
	OutputFormat string
	Verbose      bool
	NoColor      bool
}

// NewRootCommand creates the root cobra command with all global flags and subcommands.
func NewRootCommand() *cobra.Command {
	opts := &RootOptions{}

	cmd := &cobra.Command{
		Use:     "keyip",
		Short:   "KeyIP-Intelligence CLI — AI-driven IP lifecycle management for OLED materials",
		Long:    "KeyIP-Intelligence is an AI-powered intellectual property management platform\nspecialized for OLED organic materials, providing patent mining, infringement\nmonitoring, portfolio optimization, and lifecycle management capabilities.",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildDate),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return persistentPreRun(cmd, opts)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Register persistent flags.
	pf := cmd.PersistentFlags()
	pf.StringVarP(&opts.ConfigPath, "config", "c", "", "config file path (default: ./keyip.yaml)")
	pf.StringVar(&opts.LogLevel, "log-level", "info", "log level (debug, info, warn, error)")
	pf.StringVarP(&opts.OutputFormat, "output", "o", "text", "output format (text, json, table)")
	pf.BoolVarP(&opts.Verbose, "verbose", "v", false, "enable verbose output")
	pf.BoolVar(&opts.NoColor, "no-color", false, "disable colored output")
	pf.DurationVar(&opts.Timeout, "timeout", 30*time.Second, "global operation timeout")
	pf.StringVar(&opts.ServerAddr, "server", "", "API server address (default: http://localhost:8080)")

	return cmd
}

// RegisterCommands registers all subcommands with the root command.
// This is called from main.go after dependency injection.
func RegisterCommands(rootCmd *cobra.Command, deps CommandDependencies) {
	// Import necessary packages inside the function or file to avoid circular dependencies if they exist,
	// but here we rely on the main.go or the packages being available.
	// Since we are inside `cli` package, we need to ensure the service interfaces match what New*Cmd functions expect.
	// The New*Cmd functions in this package expect concrete interfaces from application layer.
	// We need to cast the interface{} placeholders to the correct types if we want to be strict,
	// or update CommandDependencies to use correct types.
	// Given the context, let's update CommandDependencies to use the correct types imported from application layer.
	// However, to keep this change minimal and focused on registration, we will assume
	// the caller passes the correct types and we might need to do type assertions if New*Cmd expect specific interfaces.
	// Actually, NewSearchCmd expects patent_mining.SimilaritySearchService.
	// NewAssessCmd expects portfolio.ValuationService.
	// NewLifecycleCmd expects lifecycle.*Service.
	// NewReportCmd expects reporting.*Service.

	// To avoid import cycles or compilation errors if those packages are not imported here, we should add imports.
	// But `cli` package already imports them in search.go, assess.go, etc. so they are available in the package scope.

	rootCmd.AddCommand(
		NewSearchCmd(deps.SimilaritySearchService, deps.Logger),
		NewAssessCmd(deps.ValuationService, deps.Logger),
		NewLifecycleCmd(
			deps.DeadlineService,
			deps.AnnuityService,
			deps.LegalStatusService,
			deps.CalendarService,
			deps.Logger,
		),
		NewReportCmd(
			deps.FTOReportService,
			deps.InfringementReportService,
			deps.PortfolioReportService,
			deps.TemplateService,
			deps.Logger,
		),
	)
}

// CommandDependencies aggregates service dependencies for CLI commands.
type CommandDependencies struct {
	Logger                    logging.Logger
	SimilaritySearchService   patent_mining.SimilaritySearchService
	ValuationService          portfolio.ValuationService
	DeadlineService           lifecycle.DeadlineService
	AnnuityService            lifecycle.AnnuityService
	LegalStatusService        lifecycle.LegalStatusService
	CalendarService           lifecycle.CalendarService
	FTOReportService          reporting.FTOReportService
	InfringementReportService reporting.InfringementReportService
	PortfolioReportService    reporting.PortfolioReportService
	TemplateService           reporting.TemplateService
}

// persistentPreRun initializes config, logger, and client, then stores CLIContext.
func persistentPreRun(cmd *cobra.Command, opts *RootOptions) error {
	cfg, err := initConfig(opts)
	if err != nil {
		return fmt.Errorf("config initialization failed: %w", err)
	}

	logger, err := initLogger(cfg, opts)
	if err != nil {
		return fmt.Errorf("logger initialization failed: %w", err)
	}

	apiClient, err := initClient(cfg, opts)
	if err != nil {
		logger.Warn("API client initialization failed, some commands may not work", logging.Err(err))
	}

	cliCtx := &CLIContext{
		Config:       cfg,
		Logger:       logger,
		Client:       apiClient,
		OutputFormat: opts.OutputFormat,
		Verbose:      opts.Verbose,
		NoColor:      opts.NoColor,
	}

	ctx := context.WithValue(cmd.Context(), cliContextKey{}, cliCtx)
	cmd.SetContext(ctx)

	return nil
}

// initConfig loads configuration with priority: flags > env > file > defaults.
func initConfig(opts *RootOptions) (*config.Config, error) {
	if opts.ConfigPath != "" {
		return config.LoadFromFile(opts.ConfigPath)
	}

	// Search default paths.
	searchPaths := []string{
		"./keyip.yaml",
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		searchPaths = append(searchPaths, filepath.Join(homeDir, ".keyip", "config.yaml"))
	}
	searchPaths = append(searchPaths, "/etc/keyip/config.yaml")

	for _, p := range searchPaths {
		if _, statErr := os.Stat(p); statErr == nil {
			return config.LoadFromFile(p)
		}
	}

	// No config file found; use defaults.
	fmt.Fprintln(os.Stderr, "Warning: no config file found, using defaults")
	return config.NewDefaultConfig(), nil
}

// initLogger creates a logger configured for CLI usage (output to stderr).
func initLogger(cfg *config.Config, opts *RootOptions) (logging.Logger, error) {
	level := logging.LevelInfo
	switch strings.ToLower(opts.LogLevel) {
	case "debug":
		level = logging.LevelDebug
	case "warn":
		level = logging.LevelWarn
	case "error":
		level = logging.LevelError
	}
	if opts.Verbose {
		level = logging.LevelDebug
	}

	logCfg := logging.LogConfig{
		Level:            level,
		Format:           "console",
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	return logging.NewLogger(logCfg)
}

// initClient creates an API client from configuration.
func initClient(cfg *config.Config, opts *RootOptions) (*client.Client, error) {
	addr := opts.ServerAddr
	if addr == "" {
		// Use HTTP config from server settings
		addr = fmt.Sprintf("http://%s:%d", cfg.Server.HTTP.Host, cfg.Server.HTTP.Port)
	}
	if addr == "" || addr == "http://:" {
		addr = "http://localhost:8080"
	}

	// NewClient requires baseURL and apiKey; use empty apiKey for now (CLI may use token auth later)
	return client.NewClient(addr, "", client.WithTimeout(opts.Timeout))
}

// GetCLIContext extracts CLIContext from a cobra command's context.
func GetCLIContext(cmd *cobra.Command) (*CLIContext, error) {
	ctx := cmd.Context()
	if ctx == nil {
		return nil, errors.NewValidationError("context", "command context is nil")
	}

	cliCtx, ok := ctx.Value(cliContextKey{}).(*CLIContext)
	if !ok || cliCtx == nil {
		return nil, errors.NewValidationError("context", "CLIContext not found in command context")
	}

	return cliCtx, nil
}

// Execute is the main entry point for the CLI application.
func Execute() error {
	rootCmd := NewRootCommand()

	if err := rootCmd.Execute(); err != nil {
		PrintError(rootCmd, err)
		return err
	}

	return nil
}

// PrintResult outputs data in the format specified by CLIContext.
func PrintResult(cmd *cobra.Command, data interface{}) error {
	cliCtx, err := GetCLIContext(cmd)
	if err != nil {
		// Fallback to JSON if context unavailable.
		return printJSON(cmd, data)
	}

	switch strings.ToLower(cliCtx.OutputFormat) {
	case "json":
		return printJSON(cmd, data)
	case "table":
		return printTable(cmd, data)
	default:
		return printText(cmd, data)
	}
}

// printJSON outputs data as indented JSON to stdout.
func printJSON(cmd *cobra.Command, data interface{}) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// printText outputs data as a simple string representation to stdout.
func printText(cmd *cobra.Command, data interface{}) error {
	switch v := data.(type) {
	case string:
		fmt.Fprintln(cmd.OutOrStdout(), v)
	case fmt.Stringer:
		fmt.Fprintln(cmd.OutOrStdout(), v.String())
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "%+v\n", v)
	}
	return nil
}

// printTable outputs data as a table if it implements the TableData interface,
// otherwise falls back to text.
func printTable(cmd *cobra.Command, data interface{}) error {
	type tableProvider interface {
		TableHeaders() []string
		TableRows() [][]string
	}

	if tp, ok := data.(tableProvider); ok {
		out := FormatTable(tp.TableHeaders(), tp.TableRows())
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}

	return printText(cmd, data)
}

// PrintError writes a formatted error message to stderr.
func PrintError(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err.Error())
}

// PrintSuccess writes a formatted success message to stdout.
func PrintSuccess(cmd *cobra.Command, msg string) {
	fmt.Fprintf(cmd.OutOrStdout(), "OK: %s\n", msg)
}

// FormatTable renders headers and rows as an aligned ASCII table.
func FormatTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Compute column widths.
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i := 0; i < len(row) && i < len(colWidths); i++ {
			if len(row[i]) > colWidths[i] {
				colWidths[i] = len(row[i])
			}
		}
	}

	var sb strings.Builder

	// Header row.
	for i, h := range headers {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(padRight(h, colWidths[i]))
	}
	sb.WriteString("\n")

	// Separator.
	for i, w := range colWidths {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(strings.Repeat("-", w))
	}
	sb.WriteString("\n")

	// Data rows.
	for _, row := range rows {
		for i := 0; i < len(headers); i++ {
			if i > 0 {
				sb.WriteString("  ")
			}
			val := ""
			if i < len(row) {
				val = row[i]
			}
			sb.WriteString(padRight(val, colWidths[i]))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// padRight pads s with spaces to the given width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// createPlaceholderCommand creates a placeholder command for when dependencies are not available.
func createPlaceholderCommand(name, description string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: description,
		Long:  description + "\n\nNote: This command requires service dependencies. Run with proper configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.NewMsg("command requires service dependencies; ensure configuration is complete")
		},
	}
}

//Personal.AI order the ending
