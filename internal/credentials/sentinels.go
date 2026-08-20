// Package credentials defines the placeholder values used to resolve secrets
// from a vault attached to a browser session.
package credentials

import "fmt"

// Field identifies a credential stored in a vault.
type Field string

const (
	Email    Field = "email"
	Username Field = "username"
	Password Field = "password"
	MFA      Field = "mfa"
)

var sentinels = map[Field]string{
	Email:    "user@example.org",
	Username: "cooljohnny1567",
	Password: "mycoolpassword",
	MFA:      "999779",
}

// Fields returns the supported vault field names in display order.
func Fields() []string {
	return []string{string(Email), string(Username), string(Password), string(MFA)}
}

// Sentinel returns the placeholder value recognized by the Notte API.
func Sentinel(field string) (string, error) {
	value, ok := sentinels[Field(field)]
	if !ok {
		return "", fmt.Errorf("unsupported vault field %q (supported: email, username, password, mfa)", field)
	}
	return value, nil
}
