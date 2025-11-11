package server

import (
	"fmt"
	"net/http"
	"stockmind/internal/agent"
	"stockmind/internal/database"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	port  int
	db    *database.Queries
	agent *agent.AgentService
}

func NewServer(dbPool *pgxpool.Pool, agent *agent.AgentService, port string) *http.Server {
	portInt, err := strconv.Atoi(port)
	if err != nil {
		portInt = 8080
	}
	NewServer := &Server{
		port:  portInt,
		db:    database.New(dbPool),
		agent: agent,
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
