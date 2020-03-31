#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import argparse
import contextlib
import json
import os
import sys

import etcd3

DRY_RUN = True


def get_nodes(etcd, prefix):
    return {
        json.loads(kv[0].decode('utf-8'))['name']
        for kv in etcd.get_prefix(prefix)
    }

def getetcd(args):
    host, _, port = args.etcd_endpoint.rpartition(':')
    return etcd3.client(host=host, port=int(port), user=args.etcd_user, password=args.etcd_password)

def getargs():
    ap = argparse.ArgumentParser()
    ap.add_argument('-p', '--pod', required=True)
    ap.add_argument('-k', '--eru-key-prefix', default='/eru')
    ap.add_argument('-e', '--etcd-endpoint', default='127.0.0.1:2379')
    ap.add_argument('--etcd-user', default=None)
    ap.add_argument('--etcd-password', default=None)
    ap.add_argument('--correct', action='store_true')
    return ap.parse_args()


class EruNode(object):

    def __init__(self, etcd, host, eru_prefix):
        self.host = host
        self.etcd = etcd
        self.eru_prefix = eru_prefix
        self.load()

    def load(self):
        self.load_node()
        self.load_containers()

    def load_containers(self):
        self.containers = EruNodeContainers(self.etcd, self.eru_prefix, self.host)

    def load_node(self):
        bytes, _ = self.etcd.get(self.key)
        if not bytes:
            raise ErrEruNodeNotExists(self.host)
        self.node = json.loads(bytes.decode('utf-8'))

    def correct(self):
        if not self.check_diff():
            return

        if self.cpu_diff:
            self.cpu_used += self.cpu_diff

        if self.memory_diff:
            self.memcap += self.memory_diff

        if self.storage_diff:
            self.storage_cap += self.storage_diff

        if not DRY_RUN:
            self.save()

    def check_diff(self):
        if not (self.cpu_diff or self.memory_diff or self.storage_diff):
            return False

        print('%s correction' % self.name)

        if self.cpu_diff < 0:
            print('\tcpuused minus %s' % abs(self.cpu_diff))
        elif self.cpu_diff > 0:
            print('\tcpuused plus %s' % self.cpu_diff)

        if self.memory_diff < 0:
            print('\tmemcap minus %s' % abs(self.memory_diff))
        elif self.memory_diff > 0:
            print('\tmemcap plus %s' % self.memory_diff)

        if self.storage_diff < 0:
            print('\tstorage_cap minus %s' % abs(self.storage_diff))
        elif self.storage_diff > 0:
            print('\tstorage_cap plus %s' % self.storage_diff)

        return True

    def save(self):
        for key in self.resource_keys:
            self.etcd.put(key, self.json)

    @property
    def json(self):
        return json.dumps(self.node)
            
    @property
    def resource_keys(self):
        return [self.key, self.pod_node_key]

    @property
    def pod_node_key(self):
        return os.path.join(self.eru_prefix, 'node/%s:pod/%s' % (self.podname, self.host))

    @property
    def podname(self):
        return self.node['podname']

    @property
    def storage_diff(self):
        return self.init_storage_cap - self.containers.storage - self.storage_cap

    @property
    def memory_diff(self):
        return self.init_memcap - self.containers.memory - self.memcap

    @property
    def cpu_diff(self):
        return self.containers.cpus - self.cpu_used

    @property
    def key(self):
        return os.path.join(self.eru_prefix, 'node', self.host)

    @property
    def name(self):
        return self.node['name']

    @property
    def cpu_used(self):
        return self.node['cpuused']

    @cpu_used.setter
    def cpu_used(self, v):
        self.node['cpuused'] = v

    @property
    def init_memcap(self):
        return self.node['init_memcap']

    @property
    def memcap(self):
        return self.node['memcap']

    @memcap.setter
    def memcap(self, v):
        self.node['memcap'] = v

    @property
    def init_storage_cap(self):
        return self.node['init_storage_cap']

    @property
    def storage_cap(self):
        return self.node['storage_cap']

    @storage_cap.setter
    def storage_cap(self, v):
        self.node['storage_cap'] = v


class EruNodeContainers(object):

    def __init__(self, etcd, prefix, host):
        self.etcd = etcd
        self.host = host
        self.eru_prefix = prefix
        self.containers = [EruContainer(json.loads(kv[0].decode('utf-8')))
                           for kv in self.etcd.get_prefix(self.prefix)]

    @property
    def storage(self):
        return sum(c.storage for c in self.containers)

    @property
    def memory(self):
        return sum(c.memory for c in self.containers)

    @property
    def cpus(self):
        return sum(c.quota for c in self.containers)

    @property
    def prefix(self):
        return os.path.join(self.eru_prefix, 'node', '%s:containers/' % self.host)


class EruContainer(object):

    def __init__(self, dic):
        self.dic = dic

    @property
    def memory(self):
        return self.dic['memory']

    @property
    def storage(self):
        return self.dic['storage']

    @property
    def quota(self):
        return self.dic['quota']


class ErrEruNodeNotExists(BaseException):

    def __init__(self, name):
        self.name = name


def main():
    args = getargs()

    global DRY_RUN
    DRY_RUN = not args.correct

    nodes_prefix = os.path.join(args.eru_key_prefix, 'node/%s:pod/' % args.pod)

    with contextlib.closing(getetcd(args)) as etcd:
        for name in get_nodes(etcd, nodes_prefix):
            if not name:
                continue

            try:
                node = EruNode(etcd, name, args.eru_key_prefix)
            except ErrEruNodeNotExists:
                print('no such node: %s' % name)
                continue
            else:
                node.correct()
        
    return 0

if __name__ == '__main__':
    sys.exit(main())
