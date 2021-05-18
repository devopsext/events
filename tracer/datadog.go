package tracer

type DataDogOptions struct {
	Host string
	Port int
}

type DataDog struct {
	options DataDogOptions
}

func NewDataDog(options DataDogOptions) *DataDog {

	return &DataDog{
		options: options,
	}
}
