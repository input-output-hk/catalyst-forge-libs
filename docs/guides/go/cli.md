# Go CLI Development with Cobra and Viper - LLM Guide

This document defines the **required** patterns for building Go CLI applications using Cobra and Viper. All code examples must follow these standards exactly.

## 1. Project Structure & Setup

### Standard CLI Project Layout

**Use this exact directory structure for all CLI applications:**

```
myapp/
├── cmd/
│   ├── root.go           # Root command and global configuration
│   ├── serve.go          # Example top-level command
│   ├── db/               # Command group directory
│   │   ├── db.go         # Parent command for group
│   │   ├── migrate.go    # Subcommand: myapp db migrate
│   │   └── seed.go       # Subcommand: myapp db seed
│   └── api/
│       ├── api.go        # Parent command for group
│       └── start.go      # Subcommand: myapp api start
├── internal/
│   └── config/
│       └── config.go     # Configuration structs
├── go.mod
├── go.sum
└── main.go               # Entry point - minimal
```

### Main Entry Point

**The `main.go` file must be minimal. It only calls the root command:**

```go
// GOOD - main.go
package main

import (
    "os"
    "myapp/cmd"
)

func main() {
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}

// BAD - logic in main
package main

import (
    "fmt"
    "myapp/cmd"
)

func main() {
    // Never put initialization logic here
    fmt.Println("Starting app...")
    config := loadConfig()
    cmd.SetConfig(config)
    cmd.Execute()
}
```

### Root Command Setup

**The `cmd/root.go` file defines the root command and exports the Execute function:**

```go
// GOOD - cmd/root.go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "MyApp is a CLI tool for managing services",
    Long: `MyApp provides comprehensive service management capabilities
including database operations, API management, and monitoring.`,
    // Root command typically has no Run function
}

// Execute is called by main.main(). It only needs to happen once.
func Execute() error {
    return rootCmd.Execute()
}

func init() {
    cobra.OnInitialize(initConfig)

    // Global flags
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
        "config file (default is $HOME/.myapp.yaml)")

    // Bind flags to viper
    viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
}

func initConfig() {
    // Config initialization - detailed in Section 3
}

// BAD - not exporting Execute
package cmd

var rootCmd = &cobra.Command{
    Use: "myapp",
    Run: func(cmd *cobra.Command, args []string) {
        // Root should rarely have Run function
        fmt.Println("Use --help for usage")
    },
}

func execute() error {  // Should be exported
    return rootCmd.Execute()
}
```

### Subcommand File Organization

**For simple subcommands, use a single file:**

```go
// GOOD - cmd/serve.go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var servePort int

var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start the application server",
    Long:  `Start the HTTP server with the configured settings.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // Command implementation
        port := viper.GetInt("port")  // Always get from viper
        return startServer(port)
    },
}

func init() {
    rootCmd.AddCommand(serveCmd)

    serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080,
        "port to run the server on")
    viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
}

// BAD - mixing multiple commands in one file
package cmd

var serveCmd = &cobra.Command{...}
var stopCmd = &cobra.Command{...}  // Should be in stop.go
var statusCmd = &cobra.Command{...} // Should be in status.go
```

### Nested Subcommand Organization

**For command groups, create a directory with parent and child commands:**

```go
// GOOD - cmd/db/db.go (parent command)
package db

import (
    "github.com/spf13/cobra"
)

// DBCmd represents the db command group
var DBCmd = &cobra.Command{
    Use:   "db",
    Short: "Database operations",
    Long:  `Perform various database operations including migrations and seeding.`,
    // No Run function - this is just a command group
}

// GOOD - cmd/db/migrate.go (child command)
package db

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var migrateCmd = &cobra.Command{
    Use:   "migrate",
    Short: "Run database migrations",
    RunE: func(cmd *cobra.Command, args []string) error {
        dbURL := viper.GetString("database.url")
        return runMigrations(dbURL)
    },
}

func init() {
    DBCmd.AddCommand(migrateCmd)

    migrateCmd.Flags().Bool("down", false, "run down migrations")
    viper.BindPFlag("migrate.down", migrateCmd.Flags().Lookup("down"))
}

// GOOD - cmd/root.go (registering the command group)
package cmd

import (
    "myapp/cmd/db"
    "github.com/spf13/cobra"
)

func init() {
    rootCmd.AddCommand(db.DBCmd)
}
```



## 2. Command Architecture

### Command Hierarchy

**Commands form a tree structure. Parent commands without Run functions act as command groups:**

```go
// GOOD - parent command without Run (command group)
var apiCmd = &cobra.Command{
    Use:   "api",
    Short: "API management commands",
    Long:  `Commands for starting, stopping, and managing the API server.`,
    // No Run or RunE - this is just a grouping command
}

// GOOD - leaf command with Run
var startCmd = &cobra.Command{
    Use:   "start",
    Short: "Start the API server",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Actual work happens here
        return startAPIServer()
    },
}

// BAD - parent command with Run function
var apiCmd = &cobra.Command{
    Use:   "api",
    Short: "API management commands",
    Run: func(cmd *cobra.Command, args []string) {
        // Parent commands should not have Run
        fmt.Println("Use 'api --help' for subcommands")
    },
}
```

### Command Lifecycle Hooks

**Cobra executes hooks in this exact order:**

```go
// Execution order for: myapp db migrate --verbose
// 1. cobra.OnInitialize functions (all registered functions)
// 2. PersistentPreRun (from closest parent that defines it)
// 3. PreRun (if defined on the command)
// 4. Run or RunE (the actual command)
// 5. PostRun (if defined)
// 6. PersistentPostRun (from closest parent)

// GOOD - proper hook usage
var rootCmd = &cobra.Command{
    Use: "myapp",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Runs before ANY command in the tree
        if err := loadConfig(); err != nil {
            return fmt.Errorf("failed to load config: %w", err)
        }
        return nil
    },
}

var dbCmd = &cobra.Command{
    Use: "db",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // This REPLACES rootCmd's PersistentPreRun for db subcommands
        // Must manually call parent if needed
        if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
            return err
        }
        // Additional db-specific setup
        return connectDatabase()
    },
}

var migrateCmd = &cobra.Command{
    Use: "migrate",
    PreRunE: func(cmd *cobra.Command, args []string) error {
        // Runs ONLY for migrate command, after PersistentPreRun
        return validateMigrationFiles()
    },
    RunE: func(cmd *cobra.Command, args []string) error {
        return runMigrations()
    },
}

// BAD - misunderstanding hook inheritance
var dbCmd = &cobra.Command{
    Use: "db",
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
        // This completely overrides parent's PersistentPreRun
        connectDatabase() // Parent's config loading won't run!
    },
}
```

### OnInitialize vs PersistentPreRun

**Use `cobra.OnInitialize` for global setup, `PersistentPreRun` for command-specific initialization:**

```go
// GOOD - proper separation of concerns
func init() {
    // OnInitialize runs before ANY command logic
    cobra.OnInitialize(initConfig, setupLogging)
}

func initConfig() {
    // Global config that ALL commands need
    viper.SetConfigName(".myapp")
    viper.AddConfigPath("$HOME")
    viper.ReadInConfig()
}

var serverCmd = &cobra.Command{
    Use: "server",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Command-specific setup that children need
        port := viper.GetInt("port")
        if port < 1 || port > 65535 {
            return fmt.Errorf("invalid port: %d", port)
        }
        return nil
    },
}

// BAD - using OnInitialize for command-specific setup
func init() {
    cobra.OnInitialize(func() {
        // This runs even when server command isn't used!
        validateServerConfig()
        openServerPort()
    })
}
```

### Command Arguments Validation

**Use Args field for validating positional arguments:**

```go
// GOOD - proper argument validation
var copyCmd = &cobra.Command{
    Use:   "copy [source] [destination]",
    Short: "Copy files",
    Args:  cobra.ExactArgs(2),  // Requires exactly 2 arguments
    RunE: func(cmd *cobra.Command, args []string) error {
        return copyFile(args[0], args[1])
    },
}

var installCmd = &cobra.Command{
    Use:   "install [packages...]",
    Short: "Install packages",
    Args:  cobra.MinimumNArgs(1),  // At least 1 argument
    RunE: func(cmd *cobra.Command, args []string) error {
        return installPackages(args)
    },
}

// Custom validation
var deployCmd = &cobra.Command{
    Use:   "deploy [environment]",
    Short: "Deploy to environment",
    Args: func(cmd *cobra.Command, args []string) error {
        if len(args) != 1 {
            return fmt.Errorf("requires exactly one environment")
        }
        if args[0] != "dev" && args[0] != "prod" {
            return fmt.Errorf("environment must be 'dev' or 'prod'")
        }
        return nil
    },
    RunE: func(cmd *cobra.Command, args []string) error {
        return deployTo(args[0])
    },
}
```

```go
// BAD - validating in Run instead of Args
var deployCmd = &cobra.Command{
    Use:   "deploy [environment]",
    Short: "Deploy to environment",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Validation should be in Args field
        if len(args) != 1 {
            return fmt.Errorf("requires exactly one environment")
        }
        return deployTo(args[0])
    },
}
```

## 3. Configuration Management

### Configuration Precedence Order

**Viper uses this exact precedence order (highest to lowest):**

```go
// Order: explicit call > flag > env > config > key/value store > default

// GOOD - understanding precedence
func init() {
    // 6. Defaults (lowest priority)
    viper.SetDefault("port", 8080)
    viper.SetDefault("database.host", "localhost")

    // 4. Config file
    viper.SetConfigName("config")
    viper.AddConfigPath(".")
    viper.ReadInConfig()

    // 3. Environment variables
    viper.SetEnvPrefix("MYAPP")
    viper.AutomaticEnv()

    // 2. Flags (bound to viper)
    rootCmd.Flags().Int("port", 0, "server port")
    viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
}

func example() {
    // 1. Explicit call (highest priority)
    viper.Set("port", 9000)

    // Result priority:
    // viper.Set("port", 9000)           -> wins
    // --port=3000 flag                  -> ignored
    // MYAPP_PORT=4000 env              -> ignored
    // port: 5000 in config.yaml        -> ignored
    // viper.SetDefault("port", 8080)   -> ignored

    port := viper.GetInt("port") // Returns 9000
}

// BAD - misunderstanding precedence
func init() {
    viper.SetDefault("port", 8080)
    cmd.Flags().Int("port", 8080, "port")  // Flag default conflicts with viper
}
```

### Binding Flags to Viper

**Always bind flags to Viper and retrieve values from Viper, not the flag:**

```go
// GOOD - bind flag and get from viper
var serverCmd = &cobra.Command{
    Use: "server",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Get from viper, not from flag
        port := viper.GetInt("server.port")
        host := viper.GetString("server.host")
        return startServer(host, port)
    },
}

func init() {
    serverCmd.Flags().Int("port", 8080, "server port")
    serverCmd.Flags().String("host", "localhost", "server host")

    // Bind flags to viper with namespace
    viper.BindPFlag("server.port", serverCmd.Flags().Lookup("port"))
    viper.BindPFlag("server.host", serverCmd.Flags().Lookup("host"))
}

// BAD - getting value from flag instead of viper
var serverCmd = &cobra.Command{
    Use: "server",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Wrong! This ignores env vars and config files
        port, _ := cmd.Flags().GetInt("port")
        return startServer(port)
    },
}

// BAD - forgetting to bind flag
func init() {
    serverCmd.Flags().Int("port", 8080, "server port")
    // Missing: viper.BindPFlag("port", serverCmd.Flags().Lookup("port"))
}
```

### Environment Variable Configuration

**Set prefix and use AutomaticEnv for automatic binding:**

```go
// GOOD - proper env var setup
func initConfig() {
    // Set prefix - all env vars must start with MYAPP_
    viper.SetEnvPrefix("MYAPP")

    // Replace . and - with _ in env var names
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

    // Automatically bind all env vars
    viper.AutomaticEnv()

    // Now these work:
    // MYAPP_PORT=3000             -> viper.GetInt("port")
    // MYAPP_DATABASE_HOST=db      -> viper.GetString("database.host")
    // MYAPP_LOG_LEVEL=debug       -> viper.GetString("log.level")
}

// GOOD - explicit env binding for specific vars
func init() {
    viper.SetEnvPrefix("MYAPP")

    // Explicit binding when key names differ
    viper.BindEnv("port")                    // Binds MYAPP_PORT
    viper.BindEnv("database.url", "DB_URL")  // Binds MYAPP_DB_URL
}

// BAD - incorrect env var naming
func init() {
    viper.AutomaticEnv()  // Missing prefix

    // These won't work as expected:
    // PORT=3000 (missing prefix)
    // myapp_port=3000 (lowercase)
    // MYAPP-PORT=3000 (dash instead of underscore)
}
```

### Config File Loading

**Set config search paths and handle missing files gracefully:**

```go
// GOOD - flexible config file loading
func initConfig() {
    if cfgFile != "" {
        // Use explicit config file
        viper.SetConfigFile(cfgFile)
    } else {
        // Search for config in standard locations
        viper.SetConfigName("config")
        viper.SetConfigType("yaml")

        // Search paths in order
        viper.AddConfigPath(".")           // Current directory
        viper.AddConfigPath("$HOME/.myapp") // Home directory
        viper.AddConfigPath("/etc/myapp")  // System directory
    }

    // Read config but don't fail if missing
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); ok {
            // Config file not found; use defaults and env
            // This is OK - not an error condition
        } else {
            // Config file found but another error occurred
            fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
        }
    }
}

// BAD - failing on missing config
func initConfig() {
    viper.SetConfigName("config")
    viper.AddConfigPath(".")

    if err := viper.ReadInConfig(); err != nil {
        // Don't fail hard on missing config!
        log.Fatal("Config file required:", err)
    }
}
```

### Setting Defaults

**Set defaults for all configuration values:**

```go
// GOOD - comprehensive defaults
func setDefaults() {
    // Server defaults
    viper.SetDefault("server.port", 8080)
    viper.SetDefault("server.host", "0.0.0.0")
    viper.SetDefault("server.timeout", 30)

    // Database defaults with nested structure
    viper.SetDefault("database.host", "localhost")
    viper.SetDefault("database.port", 5432)
    viper.SetDefault("database.maxConnections", 25)

    // Feature flags
    viper.SetDefault("features.cache", true)
    viper.SetDefault("features.rateLimit", false)
}

// BAD - setting flag defaults instead of viper defaults
func init() {
    // This creates confusion between flag defaults and viper
    cmd.Flags().String("host", "localhost", "host")  // Flag default
    viper.SetDefault("host", "0.0.0.0")              // Viper default
    // Which default wins depends on binding and flag usage
}
```

### Unmarshaling to Structs

**Define config structs and unmarshal for type safety:**

```go
// GOOD - config struct with mapstructure tags
type Config struct {
    Server   ServerConfig   `mapstructure:"server"`
    Database DatabaseConfig `mapstructure:"database"`
    Features FeatureFlags   `mapstructure:"features"`
}

type ServerConfig struct {
    Port    int    `mapstructure:"port"`
    Host    string `mapstructure:"host"`
    Timeout int    `mapstructure:"timeout"`
}

type DatabaseConfig struct {
    Host           string `mapstructure:"host"`
    Port           int    `mapstructure:"port"`
    MaxConnections int    `mapstructure:"maxConnections"`
}

func LoadConfig() (*Config, error) {
    var config Config

    // Unmarshal entire config
    if err := viper.Unmarshal(&config); err != nil {
        return nil, fmt.Errorf("unable to decode config: %w", err)
    }

    // Validate after unmarshaling
    if config.Server.Port < 1 || config.Server.Port > 65535 {
        return nil, fmt.Errorf("invalid port: %d", config.Server.Port)
    }

    return &config, nil
}

// GOOD - unmarshal sub-sections
func LoadDatabaseConfig() (*DatabaseConfig, error) {
    var dbConfig DatabaseConfig

    // Unmarshal just the database section
    if err := viper.UnmarshalKey("database", &dbConfig); err != nil {
        return nil, fmt.Errorf("unable to decode database config: %w", err)
    }

    return &dbConfig, nil
}

// BAD - not using config structs
func startServer() {
    // Scattered viper.Get calls everywhere
    port := viper.GetInt("server.port")
    host := viper.GetString("server.host")
    timeout := viper.GetInt("server.timeout")
    // Easy to typo keys, no compile-time checking
}
```

## 4. Initialization Pattern

### Complete Initialization Flow

**Use PersistentPreRunE in root command to orchestrate all initialization:**

```go
// GOOD - complete initialization pattern
var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "MyApp CLI",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // 1. Validate configuration
        if err := validateConfig(); err != nil {
            return fmt.Errorf("invalid configuration: %w", err)
        }

        // 2. Initialize dependencies
        if err := initializeServices(); err != nil {
            return fmt.Errorf("failed to initialize services: %w", err)
        }

        // 3. Setup context
        ctx := context.Background()
        ctx = context.WithValue(ctx, "config", config)
        cmd.SetContext(ctx)

        return nil
    },
}

func validateConfig() error {
    port := viper.GetInt("port")
    if port < 1 || port > 65535 {
        return fmt.Errorf("invalid port: %d", port)
    }

    dbURL := viper.GetString("database.url")
    if dbURL == "" {
        return fmt.Errorf("database.url is required")
    }

    return nil
}

func initializeServices() error {
    // Initialize only what ALL commands need
    // Command-specific init goes in their PreRunE
    return initLogger()
}

// BAD - initializing in wrong places
func init() {
    cobra.OnInitialize(func() {
        // Don't initialize services here - too early
        db = connectDB()  // What if command doesn't need DB?
    })
}

var rootCmd = &cobra.Command{
    Use: "myapp",
    Run: func(cmd *cobra.Command, args []string) {
        // Don't initialize here - children won't get it
        initServices()
    },
}
```

### Context and Dependency Propagation

**Use command.Context() to pass dependencies to child commands:**

```go
// GOOD - using context for dependencies
type ctxKey string

const (
    ctxKeyDB     ctxKey = "db"
    ctxKeyLogger ctxKey = "logger"
)

var rootCmd = &cobra.Command{
    Use: "myapp",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Initialize dependencies
        db, err := connectDB(viper.GetString("database.url"))
        if err != nil {
            return fmt.Errorf("database connection failed: %w", err)
        }

        logger := setupLogger(viper.GetString("log.level"))

        // Add to context
        ctx := context.Background()
        ctx = context.WithValue(ctx, ctxKeyDB, db)
        ctx = context.WithValue(ctx, ctxKeyLogger, logger)
        cmd.SetContext(ctx)

        return nil
    },
}

var migrateCmd = &cobra.Command{
    Use: "migrate",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Retrieve from context
        ctx := cmd.Context()
        db := ctx.Value(ctxKeyDB).(*sql.DB)
        logger := ctx.Value(ctxKeyLogger).(*slog.Logger)

        logger.Info("starting migration")
        return runMigrations(ctx, db)
    },
}

// BAD - using globals for dependencies
var (
    globalDB     *sql.DB
    globalLogger *slog.Logger
)

func init() {
    // Don't use globals
    globalDB = connectDB()
    globalLogger = setupLogger()
}
```