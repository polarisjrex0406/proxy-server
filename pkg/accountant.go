package pkg

type Accountant interface {
	Decrement(password string, byte int64) error
}
