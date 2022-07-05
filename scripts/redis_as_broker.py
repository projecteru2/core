#!/usr/bin/env python
# -*- coding: utf-8 -*-

import argparse
import sys

import etcd3
import redis


def migrate(root, etcd_client, redis_client, dry_run):
    range_start = root
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

        resp = etcd_client.kvstub.Range(
            range_request,
            etcd_client.timeout,
            credentials=etcd_client.call_credentials,
            metadata=etcd_client.metadata,
        )

        for kv in resp.kvs:
            key = kv.key.decode('utf-8')
            if not key.startswith(root + '/'):
                continue

            key = key.replace(root, '')
            value = kv.value.decode('utf-8')

            if dry_run:
                print('will set %s to %s' % (key, value))
                continue

            redis_client.set(key, value)
            print('key %s is set to %s' % (key, value))

        if not resp.more:
            break

        range_start = etcd3.utils.increment_last_byte(kv.key)


def getargs():
    ap = argparse.ArgumentParser()
    ap.add_argument('--source', dest='source_root_prefix', help='source root prefix', default='/eru')
    ap.add_argument('--etcd-host', default='127.0.0.1')
    ap.add_argument('--etcd-port', type=int, default=2379)
    ap.add_argument('--redis-host', default='127.0.0.1')
    ap.add_argument('--redis-port', type=int, default=6379)
    ap.add_argument('--redis-db', type=int, default=0)
    ap.add_argument('--dry-run', dest='dry_run', action='store_true', help='dry run, will not actually migrate')
    return ap.parse_args()


def main():
    args = getargs()

    etcd_client = etcd3.client(host=args.etcd_host, port=args.etcd_port)
    redis_client = redis.StrictRedis(host=args.redis_host, port=args.redis_port, db=args.redis_db)
    migrate(args.source_root_prefix, etcd_client, redis_client, args.dry_run)
    return 0


if __name__ == '__main__':
    sys.exit(main())
