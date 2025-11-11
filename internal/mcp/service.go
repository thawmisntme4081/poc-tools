package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func Start(ctx context.Context, protocol string) error {
	fmt.Println("Starting MCP service...")
	s := server.NewMCPServer(
		"StockMind MCP Server 🚀",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Add hello_world tool
	s.AddTool(
		mcp.NewTool("hello_world",
			mcp.WithDescription("Say hello to someone"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the person to greet"),
			),
		),
		helloHandler,
	)

	// Add add_numbers tool
	s.AddTool(
		mcp.NewTool("add_numbers",
			mcp.WithDescription("Add two numbers"),
			mcp.WithNumber("a",
				mcp.Required(),
				mcp.Description("First number"),
			),
			mcp.WithNumber("b",
				mcp.Required(),
				mcp.Description("Second number"),
			),
		),
		addNumbers,
	)

	s.AddTool(
		mcp.NewTool("get_stock_price",
			mcp.WithDescription("Get latest stock price from VCI with symbol, time frame and look back period"),
			mcp.WithString("symbol",
				mcp.Required(),
				mcp.Description("Stock symbol, e.g., HPG"),
			),
			mcp.WithString("time_frame",
				mcp.Description("Time frame, e.g., ONE_DAY, ONE_MINUTE, ONE_HOUR. Default is ONE_DAY"),
			),
			mcp.WithNumber("count_back",
				mcp.Description("Number of data points to look back. Default is 10"),
			),
		),
		getStockPrice,
	)

	// Start the server
	switch protocol {
	case "stdio":
		fmt.Println("Starting MCP server with stdio protocol")
		return server.ServeStdio(s)
	case "http":
		fmt.Println("Starting MCP server with HTTP protocol on 0.0.0.0:8090")
		h := server.NewStreamableHTTPServer(s)
		return h.Start("0.0.0.0:8090")
	default:
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

func helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!", name)), nil
}

func addNumbers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := request.RequireFloat("a")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, err := request.RequireFloat("b")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	result := a + b
	return mcp.NewToolResultStructuredOnly(map[string]float64{"result": result}), nil
}
