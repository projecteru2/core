#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import argparse
import etcd3
import functools
import json
import os

dry_run = False
record_prefix = "upgrade_"
origin_data_record_path = 'origin_data_record.data'
transferred_node_record_path = 'transferred_node_record.data'
transferred_workload_record_path = 'transferred_workload_record.data'

origin_data_recorder = None
transferred_node_recorder = None
transferred_workload_recorder = None

transferred_workloads = set()
transferred_nodes = set()


def init_recorders():
    global origin_data_recorder, origin_data_record_path
    global transferred_node_recorder, transferred_node_record_path, transferred_nodes
    global transferred_workload_recorder, transferred_workload_record_path, transferred_workloads

    origin_data_record_path = record_prefix + origin_data_record_path
    transferred_node_record_path = record_prefix + transferred_node_record_path
    transferred_workload_record_path = record_prefix + transferred_workload_record_path

    if os.path.exists(transferred_node_record_path):
        with open(transferred_node_record_path, 'r') as f:
            transferred_nodes = set(f.read().strip('\n').splitlines())

    if os.path.exists(transferred_workload_record_path):
        with open(transferred_workload_record_path, 'r') as f:
            transferred_workloads = set(f.read().strip('\n').splitlines())

    origin_data_recorder = open(origin_data_record_path, 'a')
    transferred_node_recorder = open(transferred_node_record_path, 'a')
    transferred_workload_recorder = open(transferred_workload_record_path, 'a')


def close_recorders():
    transferred_node_recorder.close()
    transferred_workload_recorder.close()
    origin_data_recorder.close()


def add_record(recorder, record):
    recorder.write('%s\n' % record)


def remove_prefix(s, prefix):
    return s[len(prefix):].lstrip('/') if s.startswith(prefix) else s


def dict_sub(d1, d2):
    if d1 is None:
        return None
    if d2 is None:
        return d1
    get = lambda d, k: d[k] if k in d else 0
    return {k: d1[k] - get(d2, k) for k in d1}


class ETCD:
    def __init__(self, client, prefix):
        """Create an instance of ETCD."""
        self.etcd = client
        self.prefix = prefix

    def get(self, key):
        if not key.startswith(self.prefix):
            key = self.prefix + key
        res = self.etcd.get(key)[0]
        if res is None:
            return None
        return res.decode('utf-8')

    def put(self, key, value):
        if not key.startswith(self.prefix):
            key = self.prefix + key
        if dry_run:
            print('put {}\n{}'.format(key, value))
            return

        origin_value = self.get(key)
        if origin_value:
            add_record(origin_data_recorder, key)
            add_record(origin_data_recorder, origin_value)

        self.etcd.put(key, value)

    def range_prefix(self, obj_prefix, fn):
        prefix = self.prefix + obj_prefix
        range_start = prefix
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

            range_response = self.etcd.kvstub.Range(
                range_request,
                self.etcd.timeout,
                credentials=self.etcd.call_credentials,
                metadata=self.etcd.metadata,
            )

            for kv in range_response.kvs:
                orig_key = kv.key.decode('utf-8')
                objname = remove_prefix(orig_key, prefix)
                fn(objname, kv.value.decode('utf-8'))

            if not range_response.more:
                break

            range_start = etcd3.utils.increment_last_byte(kv.key)


etcd: ETCD = None


class Node:
    def __init__(self, name, pod_name, meta):
        """Initializes a node transfer."""
        self.name = name
        self.pod_name = pod_name
        self.meta = json.loads(meta)

    def upgrade(self):
        cpumem_meta = self._gen_cpumem_meta()
        volume_meta = self._gen_volume_meta()
        cpumem_key = '/resource/cpumem/%s' % self.name
        volume_key = '/resource/volume/%s' % self.name
        etcd.put(cpumem_key, cpumem_meta)
        etcd.put(volume_key, volume_meta)

    def downgrade(self):
        self._load_resources_meta()
        keys = ['/node/%s' % self.name, '/node/%s:pod/%s' % (self.pod_name, self.name)]
        for key in keys:
            etcd.put(key, json.dumps(self.meta))

    def _load_cpumem_meta(self, meta):
        cpumem_meta = json.loads(meta)
        self.meta['init_cpu'] = cpumem_meta['capacity']['cpu_map']
        self.meta['cpu'] = dict_sub(cpumem_meta['capacity']['cpu_map'], cpumem_meta['usage']['cpu_map'])
        self.meta['init_memcap'] = cpumem_meta['capacity']['memory']
        self.meta['memcap'] = cpumem_meta['capacity']['memory'] - cpumem_meta['usage']['memory']
        self.meta['cpuused'] = cpumem_meta['usage']['cpu']
        self.meta['numa'] = cpumem_meta['capacity']['numa']
        self.meta['init_numa_memory'] = cpumem_meta['capacity']['numa_memory']
        self.meta['numa_memory'] = dict_sub(cpumem_meta['capacity']['numa_memory'], cpumem_meta['usage']['numa_memory'])

    def _load_resources_meta(self):
        # load cpumem resources
        cpumem_key = '/resource/cpumem/%s' % self.name
        cpumem_meta = etcd.get(cpumem_key)
        if not cpumem_meta:
            print("%s not found" % cpumem_key)
        self._load_cpumem_meta(cpumem_meta)

        # load volume resources
        volume_key = '/resource/volume/%s' % self.name
        volume_meta = etcd.get(volume_key)
        if not volume_meta:
            print("%s not found" % volume_key)
        self._load_volume_meta(volume_meta)

    def _load_volume_meta(self, meta):
        volume_meta = json.loads(meta)
        self.meta['init_volume'] = volume_meta['capacity']['volumes']
        self.meta['volume'] = volume_meta['usage']['volumes']
        self.meta['init_storage_cap'] = volume_meta['capacity']['storage']
        self.meta['storage_cap'] = volume_meta['usage']['storage']
        self.meta['volumeused'] = sum(volume_meta['usage']['volumes'].values())

    def _gen_cpumem_meta(self):
        cpumem_meta = {"capacity": {}, "usage": {}}
        cpumem_meta['capacity']['cpu_map'] = self.meta['init_cpu']
        cpumem_meta['usage']['cpu_map'] = dict_sub(self.meta['init_cpu'], self.meta['cpu'])
        cpumem_meta['capacity']['memory'] = self.meta['init_memcap']
        cpumem_meta['usage']['memory'] = self.meta['init_memcap'] - self.meta['memcap']
        cpumem_meta['capacity']['cpu'] = len(self.meta['init_cpu'])
        cpumem_meta['usage']['cpu'] = self.meta['cpuused']
        cpumem_meta['capacity']['numa'] = self.meta['numa']
        cpumem_meta['capacity']['numa_memory'] = self.meta['init_numa_memory']
        cpumem_meta['usage']['numa_memory'] = dict_sub(self.meta['init_numa_memory'], self.meta['numa_memory'])
        return json.dumps(cpumem_meta)

    def _gen_volume_meta(self):
        volume_meta = {"capacity": {}, "usage": {}}
        volume_meta['capacity']['volumes'] = self.meta['init_volume']
        volume_meta['usage']['volumes'] = dict_sub(self.meta['init_volume'], self.meta['volume'])
        volume_meta['capacity']['storage'] = self.meta['init_storage_cap']
        volume_meta['usage']['storage'] = self.meta['init_storage_cap'] - self.meta['storage_cap']
        return json.dumps(volume_meta)


class Workload:
    def __init__(self, workload_id, app_name, entry_name, node_name, meta):
        """Initializes a workload transfer."""
        self.workload_id = workload_id
        self.app_name = app_name
        self.entry_name = entry_name
        self.node_name = node_name
        self.meta = json.loads(meta)
        self.keys = ['/workloads/%s' % self.workload_id,
                     '/deploy/%s/%s/%s/%s' % (self.app_name, self.entry_name, self.node_name, self.workload_id),
                     '/node/%s:workloads/%s' % (self.node_name, self.workload_id)]

    def save(self):
        for key in self.keys:
            etcd.put(key, json.dumps(self.meta))

    def upgrade(self):
        if self.workload_id in transferred_workloads:
            return
        self._gen_resource_meta()
        self.save()

    def downgrade(self):
        if self.workload_id in transferred_workloads:
            return
        self._load_resource_meta()
        self.save()

    def _gen_resource_meta(self):
        self.meta['resource_args'] = {}
        self.meta['resource_args']['cpumem'] = {
            'cpu_request': self.meta['cpu_quota_request'],
            'cpu_limit': self.meta['cpu_quota_limit'],
            'cpu_map': self.meta['cpu'],
            'memory_request': self.meta['memory_request'],
            'memory_limit': self.meta['memory_limit'],
            "numa_node": self.meta['numa_node'],
        }
        self.meta['resource_args']['volume'] = {
            'volumes_request': self.meta['volume_request'],
            'volumes_limit': self.meta['volume_limit'],
            'volume_plan_request': self.meta['volume_plan_request'],
            'volume_plan_limit': self.meta['volume_plan_limit'],
            'storage_request': self.meta['storage_request'],
            'storage_limit': self.meta['storage_limit'],
        }
        self.meta['engine_args'] = {
            'cpu': self.meta['cpu_quota_limit'],
            'memory': self.meta['memory_limit'],
            'numa_node': self.meta['numa_node'],
            'cpu_map': self.meta['cpu'],
            'storage': self.meta['storage_limit'],
            'volume': [],
        }
        for binding in self.meta['volume_limit']:
            if not binding.startswith('AUTO'):
                self.meta['engine_args']['volume'].append(binding)

        for binding in self.meta['volume_plan_limit']:
            groups = binding.split(':')
            if len(groups) < 3:
                print("volume plan limit of %s is invalid: %s" % (self.workload_id, binding))

            dst = groups[1]
            flags = groups[2]
            device = list(self.meta['volume_plan_limit'][binding].keys())[0]
            size = self.meta['volume_plan_limit'][binding][device]
            flags = flags.replace('m', '')
            if 'o' in flags:
                flags = flags.replace('o', '').replace('r', 'ro').replace('w', 'wo')

            self.meta['engine_args']['volume'].append('%s:%s:%s:%s' % (device, dst, flags, size))

    def _load_resource_meta(self):
        self.meta['cpu_quota_request'] = self.meta['resource_args']['cpumem']['cpu_request']
        self.meta['cpu_quota_limit'] = self.meta['resource_args']['cpumem']['cpu_limit']
        self.meta['cpu'] = self.meta['resource_args']['cpumem']['cpu_map']
        self.meta['memory_request'] = self.meta['resource_args']['cpumem']['memory_request']
        self.meta['memory_limit'] = self.meta['resource_args']['cpumem']['memory_limit']
        self.meta['numa_node'] = self.meta['resource_args']['cpumem']['numa_node']
        self.meta['volume_request'] = self.meta['resource_args']['volume']['volumes_request']
        self.meta['volume_limit'] = self.meta['resource_args']['volume']['volumes_limit']
        self.meta['volume_plan_request'] = self.meta['resource_args']['volume']['volume_plan_request']
        self.meta['volume_plan_limit'] = self.meta['resource_args']['volume']['volume_plan_limit']
        self.meta['storage_request'] = self.meta['resource_args']['volume']['storage_request']
        self.meta['storage_limit'] = self.meta['resource_args']['volume']['storage_limit']


def connect_etcd(host, port):
    return etcd3.client(host=host, port=port)


def transfer_node(key, value, upgrade=True):
    if ':pod' not in key:
        return
    node_name = key.split('/')[-1]
    pod_name = key.split(':')[0].strip('/')
    if node_name in transferred_nodes:
        return

    print('transferring node %s' % node_name)
    node = Node(node_name, pod_name, value)
    if upgrade:
        node.upgrade()
    else:
        node.downgrade()
    add_record(transferred_node_recorder, node_name)


def transfer_workload(key, value, upgrade=True):
    app_name, entry_name, node_name, workload_id = key.strip('/').split('/')
    if workload_id in transferred_workloads:
        return

    print('transferring workload %s' % workload_id)
    workload = Workload(workload_id, app_name, entry_name, node_name, value)
    if upgrade:
        workload.upgrade()
    else:
        workload.downgrade()
    add_record(transferred_workload_recorder, workload_id)


def transfer(upgrade=True):
    etcd.range_prefix('/node', functools.partial(transfer_node, upgrade=upgrade))
    etcd.range_prefix('/deploy', functools.partial(transfer_workload, upgrade=upgrade))


def get_args():
    ap = argparse.ArgumentParser()
    ap.add_argument('--upgrade', action='store_true', help='upgrade to new eru-core')
    ap.add_argument('--downgrade', action='store_true', help='downgrade to old eru-core')
    ap.add_argument('--etcd-prefix', help='etcd prefix', default='/eru')
    ap.add_argument('--etcd-host', default='127.0.0.1')
    ap.add_argument('--etcd-port', type=int, default=2379)
    ap.add_argument('--dry-run', dest='dry_run', action='store_true', help='dry run, will not actually migrate')
    return ap.parse_args()


def main():
    args = get_args()
    if not args.upgrade and not args.downgrade:
        print('please specify --upgrade or --downgrade')

    global etcd, dry_run, record_prefix
    etcd = ETCD(connect_etcd(args.etcd_host, args.etcd_port), args.etcd_prefix)
    dry_run = args.dry_run
    upgrade = args.upgrade
    if not upgrade:
        record_prefix = 'downgrade'

    init_recorders()
    transfer(upgrade)


if __name__ == '__main__':
    main()
