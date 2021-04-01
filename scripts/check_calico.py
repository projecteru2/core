#!/usr/bin/env python3
import argparse
import json
import os
import re
import subprocess
import sys

import etcd3

def sh(prog, *args):
    p = subprocess.Popen((prog,)+args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, close_fds=True)
    so, se = p.communicate()
    return p.returncode, so, se


class Calico(object):

    @classmethod
    def get_weps(cls):
        items = cls.get('wep', '--all-namespaces')
        return WorkloadEndpoint.parse(items)

    @classmethod
    def get(cls, resource_type, *args):
        rc, so, se = cls.ctl('get', resource_type, '-o', 'json', *args)
        if rc:
            raise ValueError('get %s failed: %s; %s; %s' % (resource_type, rc, so, se))
        return json.loads(so.decode('utf-8'))['items']

    @classmethod
    def ctl(cls, subcommand, *args):
        return sh('calicoctl', subcommand, *args)


class WorkloadEndpoint(object):

    def __init__(self, raw_dict):
        self.dict = raw_dict

    @classmethod
    def parse(cls, items):
        weps = []
        for elem in items:
            weps.append(WorkloadEndpoint(elem))
        return weps

    def has_belong_eru(self, eru):
        return any(eru.has_ip(ip) for ip in self.ips)

    @property
    def name(self):
        return self.dict['metadata']['name']
    
    @property
    def namespace(self):
        return self.dict['metadata']['namespace']

    @property
    def node(self):
        return self.dict['spec']['node']

    @property
    def interface(self):
        return self.dict['spec']['interfaceName']

    @property
    def ips(self):
        return [cidr.split('/')[0] for cidr in self.dict['spec']['ipNetworks']]


class Eru(object):

    def __init__(self, etcd, root_prefix):
        self.etcd = etcd
        self.root_prefix = os.path.join('/', root_prefix.lstrip('/'))
        self.ips = []

    def watch_ips(self):

        ipre = re.compile(r'^(\d+\.\d+\.\d+\.\d+)')

        def parse(key, value):
            for _, ip in json.loads(value).get('networks', {}).items():
                match = ipre.search(ip)
                if not match:
                    continue
                self.ips.append(match.group(1))

        self.etcd.get_prefix(self.workload_status_prefix, parse)

    @property
    def workload_status_prefix(self):
        # the ended with '/' is necessary to avoid 'status:node'
        return os.path.join(self.root_prefix, 'status/')

    def has_ip(self, ip):
        return ip in self.ips


class ETCD(object):

    def __init__(self, cli):
        self.cli = cli

    @classmethod
    def connect(cls, host, port):
        cli = etcd3.client(host=host, port=port)
        return ETCD(cli)

    def get_prefix(self, prefix, fn):
        start = prefix
        end = etcd3.utils.increment_last_byte(etcd3.utils.to_bytes(start))

        while 1:
            req = etcd3.etcdrpc.RangeRequest()
            req.key = etcd3.utils.to_bytes(start)
            req.keys_only = False
            req.range_end = etcd3.utils.to_bytes(end)
            req.sort_order = etcd3.etcdrpc.RangeRequest.ASCEND
            req.sort_target = etcd3.etcdrpc.RangeRequest.KEY
            req.serializable = True
            req.limit = 1000

            resp = self.cli.kvstub.Range(
                req,
                self.cli.timeout,
                credentials=self.cli.call_credentials,
                metadata=self.cli.metadata,
            )

            for kv in resp.kvs:
                key = kv.key.decode('utf-8')
                fn(kv.key.decode('utf-8'), kv.value.decode('utf-8'))

            if not resp.more:
                return

            start = etcd3.utils.increment_last_byte(kv.key)


def print_dangling(wep):
    print('%s/%s is dangling' % (wep.namespace, wep.name))

def get_args():
    ap = argparse.ArgumentParser()
    ap.add_argument('-e', '--eru-etcd-endpoints', help='the ERU ETCD endpoints', default='127.0.0.1:2379')
    ap.add_argument('-p', '--eru-etcd-prefix', help='the ERU ETCD root prefix', required=True)
    return ap.parse_args()

def main():
    args = get_args()

    host, _, port = args.eru_etcd_endpoints.split(',')[0].partition(':')
    port = int(port) if port else 2379
    etcd = ETCD.connect(host, port)

    global eru
    eru = Eru(etcd, args.eru_etcd_prefix)
    eru.watch_ips()

    for wep in Calico.get_weps():
        if 'yavirt-cali-gw' in wep.interface or wep.has_belong_eru(eru):
            continue

        print_dangling(wep)

    return 0

if __name__ == '__main__':
    sys.exit(main())
