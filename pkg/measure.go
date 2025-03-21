package pkg

type Measure interface {
	IncReadBytes(password string, bytes int64) error
	IncWriteBytes(password string, bytes int64) error
	IncRequest(password string) error
}
