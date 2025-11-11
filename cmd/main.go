package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stockmind/internal/agent"
	"stockmind/internal/database"
	"stockmind/internal/mcp"
	"stockmind/internal/server"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
)

func main() {
	// Load .env file first
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Warning: Failed to load .env file: %v\n", err)
	}

	app := cli.Command{
		Name:    "stock_mind",
		Usage:   "StockMind is an AI-powered assistant designed to simplify access to financial information and insights about the Vietnamese stock market.",
		Version: "1.0.0",
		Commands: []*cli.Command{
			{
				Name:  "server",
				Usage: "Run the StockMind application",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "port",
						Value: "8080",
						Usage: "Port to run the server on",
					},
					&cli.StringFlag{
						Name:    "mcp-protocol",
						Aliases: []string{"p"},
						Usage:   "MCP protocol (stdio, http)",
						Value:   "stdio",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					port := cmd.String("port")
					mcpProtocol := cmd.String("mcp-protocol")

					runContext, shutdown, err := runServer(ctx, port, mcpProtocol)
					if err != nil {
						log.Printf("Failed to run server: %v", err)
						return err
					}

					signalCtx, cancel := signal.NotifyContext(runContext, syscall.SIGINT, syscall.SIGTERM)
					defer cancel()

					<-signalCtx.Done()
					shutdown()
					return nil
				},
			},
			{
				Name:  "mcp",
				Usage: "Run the MCP server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "protocol",
						Aliases: []string{"p"},
						Usage:   "Protocol to use (stdio, http). http means Streamable HTTP protocol",
						Value:   "stdio",
						Sources: cli.ValueSourceChain{
							Chain: []cli.ValueSource{
								cli.EnvVar("MCP_PROTOCOL"),
							},
						},
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					protocol := cmd.String("protocol")
					return runMCP(ctx, protocol)
				},
			},
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func runMCP(ctx context.Context, protocol string) error {
	log.Printf("Running MCP server with protocol: %s", protocol)
	return mcp.Start(ctx, protocol)
}

func runServer(ctx context.Context, port string, mcpProtocol string) (context.Context, func(), error) {
	log.Printf("Running server on port: %s", port)

	var mcpShutdown func()
	if mcpProtocol == "http" {
		// Create MCP service and HTTP server
		log.Printf("Initializing MCP server with HTTP protocol on 0.0.0.0:8081")
		err := mcp.Start(ctx, mcpProtocol)
		if err != nil {
			log.Printf("Failed to start MCP: %v", err)
			return nil, nil, err
		}
	}

	dbUrl := "postgres://" + os.Getenv("DB_USERNAME") + ":" + url.QueryEscape(os.Getenv("DB_PASSWORD")) + "@" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + "/" + os.Getenv("DB_DATABASE") + "?sslmode=disable"

	// Create a database connection pool
	poolConfig, err := pgxpool.ParseConfig(dbUrl)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse database URL: %v", err)
	}
	poolConfig.MaxConns = 10

	dbPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create database pool: %v", err)
	}

	// Test the database connection
	err = dbPool.Ping(ctx)
	if err != nil {
		log.Printf("Failed to ping database: %v", err)
		return nil, nil, err
	}

	// Run Migration
	err = database.MigrateDB(dbPool)
	if err != nil {
		log.Println("Failed to migrate database", "error", err)
		return nil, nil, err
	}
	log.Println("Database connection established")

	// Create an agent service
	agent, err := agent.NewService(ctx, dbPool, database.ModelProviderOpenAI)
	if err != nil {
		log.Println("Failed to create agent service", "error", err)
		return nil, nil, err
	}

	// Create a server for the application
	server := server.NewServer(dbPool, agent, port)
	runContext, cancel := context.WithCancel(ctx)

	// Create a done channel to signal when the shutdown is complete
	stopCh := make(chan struct{})

	go func() {
		defer close(stopCh)

		log.Printf("Server starting on port: %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("unexpected server closure: %v", err)
			cancel()
		}
	}()

	return runContext, func() {
		// Create shutdown context with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		// Shutdown MCP server first if it was started
		if mcpShutdown != nil {
			log.Printf("Shutting down MCP server...")
			mcpShutdown()
		}

		// Then shutdown HTTP server
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("stopping server: %v", err)
		}

		<-stopCh
	}, nil
}
