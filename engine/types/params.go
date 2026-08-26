package types

type Params struct {
	Nodename string
	Endpoint string
}

func NewParams(nodename, endpoint string) *Params {
	return &Params{
		Nodename: nodename,
		Endpoint: endpoint,
	}
}

func (p *Params) CacheKey() string {
	return p.Endpoint
}
