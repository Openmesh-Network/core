package p2p

// DHTValidator is used to validate the record in the DHT
type DHTValidator struct{}

// NewDHTValidator initialise a new DHT validator
func NewDHTValidator() *DHTValidator {
    return &DHTValidator{}
}

// Validate is to validate a key in the DHT
// TODO It always returns nil (i.e. valid) by now. Please add validation mechanism if needed.
func (v *DHTValidator) Validate(key string, value []byte) error {
    // nil = valid
    return nil
}

// Select returns the index of the best value and nil, or -1 and an error if none are valid
func (v *DHTValidator) Select(key string, values [][]byte) (int, error) {
    return 0, nil
}
