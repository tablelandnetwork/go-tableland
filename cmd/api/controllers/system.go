package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/pkg/errors"
)

// SystemController defines the HTTP handlers for interacting with system operations
type SystemController struct {
	systemService system.SystemService
}

// NewSystemController creates a new SystemController
func NewSystemController(svc system.SystemService) *SystemController {
	return &SystemController{svc}
}

// GetTable handles the GET /tables/{uuid} call
func (c *SystemController) GetTables(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)

	fmt.Println(vars["uuid"])
	uuid, err := uuid.Parse(vars["uuid"])
	if err != nil {
		rw.WriteHeader(http.StatusUnprocessableEntity)
		// TODO: log err
		json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid uuid"})
		return
	}

	metadata, err := c.systemService.GetTableMetadata(uuid)
	if err != nil {
		rw.WriteHeader(http.StatusNotFound)
		// TODO: log err
		json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to fetch metadata"})
		return
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(metadata)
}
