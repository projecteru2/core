# coding: utf-8

import click
import core_pb2 as pb
from grpc.beta import implementations
from grpc.framework.interfaces.face.face import AbortionError


def _get_stub(ctx):
    try:
        channel = implementations.insecure_channel('localhost', 5000)
    except Exception:
        click.echo(click.style('error getting channel', fg='red', bold=True))
        ctx.exit(-1)

    if not channel:
        click.echo(click.style('error getting stub', fg='red', bold=True))
        ctx.exit(-1)
    return pb.beta_create_CoreRPC_stub(channel)


@click.group()
@click.pass_context
def cli(ctx):
    pass


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


@cli.command('pod:create')
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


if __name__ == '__main__':
    cli()
