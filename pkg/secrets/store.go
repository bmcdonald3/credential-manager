package secrets

// SecretStore defines an abstraction over secret persistence backends.
type SecretStore interface {
	Read(id string) (string, error)
	Write(id string, data string) error
	Update(id string, data string) error
	Delete(id string) error
}
