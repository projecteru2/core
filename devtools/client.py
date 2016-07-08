#!/usr/bin/env python
# -*- coding: utf-8 -*-

import click
from grpc.beta import implementations
from grpc.framework.interfaces.face.face import AbortionError

import core_pb2 as pb


def _get_stub(ctx):
    try:
        channel = implementations.insecure_channel(ctx.obj['grpc_host'], ctx.obj['grpc_port'])
    except Exception:
        click.echo(click.style('error getting stub', fg='red', bold=True))
        ctx.exit(-1)
    return pb.beta_create_CoreRPC_stub(channel)


@click.group()
@click.option('--grpc-host', default='localhost', show_default=True)
@click.option('--grpc-port', default=5000, show_default=True, type=int)
@click.pass_context
def cli(ctx, grpc_host, grpc_port):
    ctx.obj['grpc_host'] = grpc_host
    ctx.obj['grpc_port'] = grpc_port


@cli.command('pod:list')
@click.pass_context
def list_pods(ctx):
    stub = _get_stub(ctx)
    try:
        r = stub.ListPods(pb.Empty(), 5)
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    for p in r.pods:
        click.echo(p)


@cli.command('pod:add')
@click.argument('name')
@click.argument('desc')
@click.pass_context
def create_pod(ctx, name, desc):
    stub = _get_stub(ctx)
    opts = pb.AddPodOptions(name=name, desc=desc)

    try:
        pod = stub.AddPod(opts, 5)
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    if not pod:
        click.echo(click.style('error creating pod', fg='red', bold=True))
        ctx.exit(-1)

    click.echo(click.style('create pod %s successfully' % pod, fg='green'))


@cli.command('pod:get')
@click.argument('name')
@click.pass_context
def get_pod(ctx, name):
    stub = _get_stub(ctx)
    opts = pb.GetPodOptions(name=name)

    try:
        pod = stub.GetPod(opts, 5)
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    click.echo(pod)


@cli.command('pod:nodes')
@click.argument('name')
@click.pass_context
def get_pod_nodes(ctx, name):
    stub = _get_stub(ctx)
    opts = pb.ListNodesOptions(podname=name)

    try:
        r = stub.ListPodNodes(opts, 5)
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    for node in r.nodes:
        click.echo(node)


@cli.command('node:get')
@click.argument('podname')
@click.argument('nodename')
@click.pass_context
def get_node(ctx, podname, nodename):
    stub = _get_stub(ctx)
    opts = pb.GetNodeOptions(podname=podname, nodename=nodename)

    try:
        node = stub.GetNode(opts, 5)
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    click.echo(node)


@cli.command('node:add')
@click.argument('nodename')
@click.argument('endpoint')
@click.argument('podname')
@click.option('--public', '-p', is_flag=True)
@click.pass_context
def add_node(ctx, nodename, endpoint, podname, public):
    stub = _get_stub(ctx)
    opts = pb.AddNodeOptions(nodename=nodename,
                             endpoint=endpoint,
                             podname=podname,
                             public=public)

    try:
        node = stub.AddNode(opts, 5)
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    click.echo(node)


@cli.command('build')
@click.argument('repo')
@click.argument('version')
@click.argument('uid')
@click.option('--artifact', default='')
@click.pass_context
def build_image(ctx, repo, version, uid, artifact):
    # artifact = 'http://gitlab.ricebook.net/api/v3/projects/245/builds/1815/artifacts'
    stub = _get_stub(ctx)
    opts = pb.BuildImageOptions(repo=repo, version=version, uid=uid, artifact=artifact)

    try:
        for m in stub.BuildImage(opts, 3600):
            if m.error:
                click.echo(click.style(m.error, fg='red'), nl=False)
            elif m.stream:
                click.echo(click.style(m.stream), nl=False)
            elif m.status:
                click.echo(click.style(m.status))
                if m.progress:
                    click.echo(click.style(m.progress))
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    click.echo(click.style('done', fg='green'))


@cli.command('deploy')
@click.pass_context
def create_container(ctx):
    stub = _get_stub(ctx)
    specs = """appname: "test-ci"
entrypoints:
  web:
    cmd: "python run.py"
    ports:
      - "5000/tcp"
  restart:
    cmd: "python test_restart.py"
    restart: "always"
  log:
    cmd: "python log.py"
  fullcpu:
    cmd: "python fullcpu.py"
    restart: "always"
build:
  - "pip install -r requirements.txt -i https://pypi.doubanio.com/simple/"
base: "hub.ricebook.net/base/alpine:python-2016.04.24"
"""
    opts = pb.DeployOptions(specs=specs,
                            appname='test-ci',
                            image='hub.ricebook.net/test-ci:966fd83',
                            podname='dev',
                            entrypoint='web',
                            cpu_quota=1,
                            count=2,
                            networks={'zzz': ''}, # 如果不需要指定IP就写空字符串, 写其他的错误的格式会报错失败
                            env=['ENV_A=1', 'ENV_B=2'])

    try:
        for m in stub.CreateContainer(opts, 3600):
            if m.error:
                click.echo(click.style(m.error, fg='red', bold=True))
            else:
                click.echo(m)
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    click.echo(click.style('done', fg='green'))


@cli.command('container:remove')
@click.argument('ids', nargs=-1)
@click.pass_context
def remove_container(ctx, ids):
    stub = _get_stub(ctx)
    ids = pb.ContainerIDs(ids=[pb.ContainerID(id=i) for i in ids])

    try:
        for m in stub.RemoveContainer(ids, 3600):
            click.echo('%s: success %s, message: %s' % (m.id, m.success, m.message))
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    click.echo(click.style('done', fg='green'))


@cli.command('container:upgrade')
@click.argument('ids', nargs=-1)
@click.argument('image')
@click.pass_context
def upgrade_container(ctx, ids, image):
    stub = _get_stub(ctx)
    opts = pb.UpgradeOptions(ids=[pb.ContainerID(id=i) for i in ids], image=image)

    try:
        for m in stub.UpgradeContainer(opts, 3600):
            click.echo('[%s] success:%s, id:%s, name:%s, error:%s' % (m.id, m.success, m.new_id, m.new_name, m.error))
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)

    click.echo(click.style('done', fg='green'))


@cli.command('container:get')
@click.argument('ids', nargs=-1)
@click.pass_context
def get_containers(ctx, ids):
    stub = _get_stub(ctx)
    ids = pb.ContainerIDs(ids=[pb.ContainerID(id=i) for i in ids])

    try:
        cs = stub.GetContainers(ids, 3600)
        for c in cs.containers:
            click.echo('%s, podname: %s, nodename: %s' % (c.id, c.podname, c.nodename))
            click.echo('%s, info: %s' % (c.id, c.info))
    except AbortionError as e:
        click.echo(click.style('abortion error: %s' % e.details, fg='red', bold=True))
        ctx.exit(-1)


if __name__ == '__main__':
    cli(obj={})
