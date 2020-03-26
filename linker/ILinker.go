package linker

// linker
type ILinker interface {
	SendMsg(router string, data []byte)
	GetSubRounter() string
	Run() error
	GetConnection() interface{}
	Done()
}
