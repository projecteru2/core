package types

// Lambda .
type Lambda struct {
	ID string `json:"id"`
}

// LambdaStatus .
type LambdaStatus struct {
	*Lambda
	Error error
}
