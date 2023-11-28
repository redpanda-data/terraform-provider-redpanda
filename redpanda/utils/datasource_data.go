package utils

// DatasourceData is used to pass data and dependencies to data implementations
type DatasourceData struct {
	ClientID     string
	ClientSecret string
	AuthToken    string
	Version      string
}

// TODO add cloud provider and region as values to persist
