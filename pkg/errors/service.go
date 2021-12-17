package errors

// ServiceError should be used to return error messages in JSON format.
type ServiceError struct {
	Message string `json:"message"`
}
