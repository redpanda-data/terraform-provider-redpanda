package utils

// ResourceData is used to pass data and dependencies to resource implementations
type ResourceData struct {
	ClientID     string
	ClientSecret string
	AuthToken    string
	Version      string
}

// TODO add cloud provider and region as values to persist
