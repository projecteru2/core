package types

import "path/filepath"

type Processing struct {
	Appname   string
	Entryname string
	Nodename  string
	Ident     string
}

func (o DeployOptions) NewProcessing(nodename string) *Processing {
	return &Processing{
		Appname:   o.Name,
		Entryname: o.Entrypoint.Name,
		Nodename:  nodename,
		Ident:     o.ProcessIdent,
	}
}

func (p Processing) BaseKey() string {
	return filepath.Join(p.Appname, p.Entryname, p.Nodename, p.Ident)
}
