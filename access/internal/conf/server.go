package conf

type ServerInstance struct {
	Id       string
	Name     string
	Version  string
	Metadata map[string]string
	Endpoint string
}
