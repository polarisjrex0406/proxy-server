package pkg

type UsernameParser interface {
	Parse(username []byte, req *Request) (err error)
}
