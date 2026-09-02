package etcdv3

import "github.com/projecteru2/core/store/etcdv3/meta"

func kvOf(m *Mercury) meta.KV {
	return m.KV.(*etcdKV).kv
}
