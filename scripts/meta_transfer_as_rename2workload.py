#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import argparse
import functools
import json
import os
import sys

import etcd3

def remove_prefix(s, prefix):
    return s[len(prefix):].lstrip('/') if s.startswith(prefix) else s

def range_prefix(meta, obj_prefix, fn):
    etcd = meta.etcd
    orig_prefix = os.path.join(meta.orig_root_prefix, obj_prefix)
    range_start = orig_prefix
    range_end = etcd3.utils.increment_last_byte(
        etcd3.utils.to_bytes(range_start)
    )

    while True:
        range_request = etcd3.etcdrpc.RangeRequest()
        range_request.key = etcd3.utils.to_bytes(range_start)
        range_request.keys_only = False
        range_request.range_end = etcd3.utils.to_bytes(range_end)
        range_request.sort_order = etcd3.etcdrpc.RangeRequest.ASCEND
        range_request.sort_target = etcd3.etcdrpc.RangeRequest.KEY
        range_request.serializable = True
        range_request.limit = 1000

        range_response = etcd.kvstub.Range(
            range_request,
            etcd.timeout,
            credentials=etcd.call_credentials,
            metadata=etcd.metadata,
        )

        for kv in range_response.kvs:
            orig_key = kv.key.decode('utf-8')
            objname = remove_prefix(orig_key, orig_prefix)
            new_key = fn(objname, kv.value.decode('utf-8'))
            if new_key:
                print('convert %s to %s' % (orig_key, new_key))

        if not range_response.more:
            break

        range_start = etcd3.utils.increment_last_byte(kv.key)

def range_prefix2(meta, obj_prefix, fn):
    orig_prefix = os.path.join(meta.orig_root_prefix, obj_prefix)

    for orig_value, orig_meta in meta.etcd.get_prefix(orig_prefix):
        orig_key = orig_meta.key.decode('utf-8')
        objname = remove_prefix(orig_key, orig_prefix)

        new_key = fn(objname, orig_value.decode('utf-8'))
        if new_key:
            print('convert %s to %s' % (orig_key, new_key))


class Pod(object):

    def __init__(self, meta):
        """Initializes a pod transfer."""
        self.meta = meta
        self.pod_prefix = 'pod/info'
        self.range_prefix = functools.partial(range_prefix, self.meta)

    def trans(self):
        self.range_prefix(self.pod_prefix, self._trans)

    def _trans(self, podname, orig_value):
        new_key = os.path.join(self.meta.new_root_prefix, self.pod_prefix, podname)
        self.meta.etcd.put(new_key, orig_value)
        return new_key


class Node(object):

    def __init__(self, meta):
        """Initializes a node transfer."""
        self.meta = meta
        self.info_prefix = 'node'
        self.range_prefix = functools.partial(range_prefix, self.meta, self.info_prefix)
        self.nodes = {}

    def trans(self):
        self.range_prefix(self._trans_info)
        self.range_prefix(self._trans_pod)
        self.range_prefix(self._trans_cert)
        self.range_prefix(self._trans_workload)

    def _trans_info(self, nodename, orig_value):
        # skipping extra info.
        if ':' in nodename:
            return

        self.nodes[nodename] = json.loads(orig_value)

        new_key = os.path.join(self.meta.new_root_prefix, self.info_prefix, nodename)
        self.meta.etcd.put(new_key, orig_value)
        return new_key

    def _trans_pod(self, node_pod_pair, orig_value):
        # parsering node-pod-pair itself only
        if ':pod/' not in node_pod_pair:
            return

        podname, _, nodename = node_pod_pair.partition(':pod/')
        if not (podname and nodename):
            raise ValueError('invalid podname or nodename for %s' % node_pod_pair)

        new_key = os.path.join(self.meta.new_root_prefix, self.info_prefix, '%s:pod' % podname, nodename)
        self.meta.etcd.put(new_key, orig_value)
        return new_key

    def _trans_cert(self, cert_key, orig_value):
        nodename, _, cert_type = cert_key.partition(':')

        # parsering orig_key which ends with :ca, :cert, :key only
        if cert_type not in ('ca', 'cert', 'key'):
            return

        new_key = os.path.join(self.meta.new_root_prefix, self.info_prefix, '%s:%s' % (nodename, cert_type))
        self.meta.etcd.put(new_key, orig_value)
        return new_key

    def _trans_workload(self, node_wrk_pair, orig_value):
        nodename, _, wrk_id = node_wrk_pair.partition(':containers/')

        # parsering orig_key which belongs node-workload pair only.
        if not (nodename and wrk_id):
            return

        new_key = os.path.join(self.meta.new_root_prefix, self.info_prefix, '%s:workloads' % nodename, wrk_id)
        wrk = Workload.conv(orig_value, self)
        self.meta.etcd.put(new_key, json.dumps(wrk))
        return new_key

    def get_numa_node(self, cpumap, nodename):
        """Ref core types/core.go GetNUMANode func."""
        numa_node_id = ""

        node = self.nodes.get(nodename)
        if not node:
            raise ValueError('invalid nodename %s' % nodename)

        numa = node.get('numa')
        if not numa:
            return numa_node_id

        for cpu_id in cpumap:
            mem_node = numa.get(cpu_id)
            if not mem_node:
                continue

            if numa_node_id == '':
                numa_node_id = mem_node
            elif numa_node_id != mem_node:
                numa_node_id = ''

        return numa_node_id


class Workload(object):

    def __init__(self, meta, node_transfer):
        """Initializes a workload transfer."""
        self.meta = meta
        self.container_prefix = 'containers'
        self.wrk_prefix = 'workloads'
        self.deploy_prefix = 'deploy'
        self.range_prefix = functools.partial(range_prefix, self.meta)
        self.node_transfer = node_transfer

    def trans(self):
        self.range_prefix(self.container_prefix, self._trans_container)
        self.range_prefix(self.deploy_prefix, self._trans_deploy)

    def _trans_container(self, wrk_id, orig_value):
        new_key = os.path.join(self.meta.new_root_prefix, self.wrk_prefix, wrk_id)
        wrk = self.conv(orig_value, self.node_transfer)
        self.meta.etcd.put(new_key, json.dumps(wrk))
        return new_key

    def _trans_deploy(self, deploy_key, orig_value):
        parts = deploy_key.split('/')
        if len(parts) != 4:
            print('invalid deploy key: %s' % deploy_key)
            return

        appname, entrypoint, nodename, wrk_id = parts

        new_key = os.path.join(self.meta.new_root_prefix, self.deploy_prefix, appname, entrypoint, nodename, wrk_id)
        wrk = self.conv(orig_value, self.node_transfer)
        self.meta.etcd.put(new_key, json.dumps(wrk))

        return new_key

    @classmethod
    def conv(cls, orig_value, node_transfer):

        def delete(*keys):
            for k in keys:
                try:
                    del dic[k]
                except KeyError:
                    pass

        del_keys = set()
        def get(new_field, orig_field, transit_field, default=None):
            value = None

            # don't use dic.get(new_field, dic[orig_field]),
            # due to there isn't orig_field but has new_field.
            if new_field in dic:
                value = dic[new_field]
            elif transit_field in dic:
                value = dic[transit_field]
            else:
                if default is None:
                    value = dic[orig_field]
                else:
                    value = dic.get(orig_field, default)

            del_keys.update({orig_field, transit_field})

            return value

        dic = json.loads(orig_value)
        dic['cpu'] = get('CPU', 'cpu', 'CPU')

        dic.update(dict(
            create_time=1553990400,
            cpu_quota_request=get('cpu_quota_request', 'quota', 'CPUQuotaRequest'),
            cpu_quota_limit=get('cpu_quota_limit', 'quota', 'CPUQuotaLimit'),
            memory_request=get('memory_request', 'memory', 'MemoryRequest'),
            memory_limit=get('memory_limit', 'memory', 'MemoryLimit'),
            volume_request=get('volume_request', 'volumes', 'VolumeRequest', default=[]),
            volume_limit=get('volume_limit', 'volumes', 'VolumeLimit', default=[]),
            volume_plan_request=get('volume_plan_request', 'volume_plan', 'VolumePlanRequest', default={}),
            volume_plan_limit=get('volume_plan_limit', 'volume_plan', 'VolumePlanLimit', default={}),
            volume_changed=dic.get('volume_changed', False),
            storage_request=get('storage_request', 'storage', 'StorageRequest'),
            storage_limit=get('storage_limit', 'storage', 'StorageLimit'),
        ))

        dic['numa_node'] = ''
        if dic['cpu'] and node_transfer:
            numa_node = node_transfer.get_numa_node(dic['cpu'], dic['nodename'])
            dic['numa_node'] = dic.get('NUMANode', numa_node)

        # don't removing *cpu* from the original dict.
        try:
            del_keys.remove('cpu')
        except KeyError:
            pass

        del_keys.update({'softlimit', 'VolumeChanged', 'NUMANode'})
        delete(*list(del_keys))

        return dic


class Transfer(object):

    def __init__(self, etcd, orig_root_prefix, new_root_prefix):
        """Initializes a transfer which includes common utilities."""
        self.etcd = etcd
        self.orig_root_prefix = orig_root_prefix
        self.new_root_prefix = new_root_prefix

    def trans(self):
        Pod(self).trans()

        node_transfer = Node(self)
        node_transfer.trans()

        Workload(self, node_transfer).trans()

def getargs():
    ap = argparse.ArgumentParser()
    ap.add_argument('-o', dest='orig_root_prefix', help='original prefix', default='/eru')
    ap.add_argument('-n', dest='new_root_prefix', help='new prefix', default='/eru2')
    ap.add_argument('--etcd-host', default='127.0.0.1')
    ap.add_argument('--etcd-port', type=int, default=2379)
    return ap.parse_args()

def connect_etcd(host, port):
    return etcd3.client(host=host, port=port)

def main():
    args = getargs()
    etcd = connect_etcd(args.etcd_host, args.etcd_port)
    trans = Transfer(etcd, args.orig_root_prefix, args.new_root_prefix)
    trans.trans()
    return 0

if __name__ == '__main__':
    sys.exit(main())
