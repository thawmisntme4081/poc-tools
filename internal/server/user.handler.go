package server

import (
	"encoding/json"
	"net/http"
	"stockmind/internal/database"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var req database.CreateUserParams

	// Parse JSON request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Email == "" {
		http.Error(w, "Name and email are required", http.StatusBadRequest)
		return
	}

	// Create user object
	user := &database.CreateUserParams{
		Name:     req.Name,
		Email:    req.Email,
		Provider: req.Provider,
	}

	// Create user in database
	if _, err := s.db.CreateUser(r.Context(), *user); err != nil {
		http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// Return created user
	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (s *Server) GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	// Get users from database
	users, err := s.db.GetUsers(r.Context())
	if err != nil {
		http.Error(w, "Failed to get users: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Return users list
	if err := json.NewEncoder(w).Encode(users); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (s *Server) GetUserByIDHandler(w http.ResponseWriter, r *http.Request) {
	// Get user ID from URL parameter
	id := chi.URLParam(r, "id")

	// Get user from database
	user, err := s.db.GetUserByID(r.Context(), uuid.Must(uuid.Parse(id)))
	if err != nil {
		http.Error(w, "User not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Return user
	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (s *Server) UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	// Get user ID from URL parameter
	id := chi.URLParam(r, "id")

	var req database.UpdateUserParams

	// Parse JSON request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Check if user exists
	existingUser, err := s.db.GetUserByID(r.Context(), uuid.Must(uuid.Parse(id)))
	if err != nil {
		http.Error(w, "User not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Update user fields (only update provided fields)
	user := &database.UpdateUserParams{
		ID:       uuid.Must(uuid.Parse(id)),
		Name:     existingUser.Name,
		Email:    existingUser.Email,
		Provider: existingUser.Provider,
	}

	// Update fields if provided
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}

	// Update user in database
	if err := s.db.UpdateUser(r.Context(), *user); err != nil {
		http.Error(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Return updated user
	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (s *Server) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	// Get user ID from URL parameter
	id := chi.URLParam(r, "id")

	// Check if user exists
	_, err := s.db.GetUserByID(r.Context(), uuid.Must(uuid.Parse(id)))
	if err != nil {
		http.Error(w, "User not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Delete user from database
	if err := s.db.DeleteUser(r.Context(), uuid.Must(uuid.Parse(id))); err != nil {
		http.Error(w, "Failed to delete user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.WriteHeader(http.StatusNoContent)
}
