package pkg

type Measure interface {
	IncReadBytes(password string, bytes int64) error
	IncWriteBytes(password string, bytes int64) error
	IncRequest(password string) error
	LogThreads(password string, threads int64) error
	CountError(password, err string) error
	LogAdoptedFeature(password, feature string) error
}
