from __future__ import annotations
import sys
import yaml
import json
import typing
import operator


def itemsetter(obj, key):
    return lambda v: operator.setitem(obj, key, v)


def is_list_without_dict(value: typing.Any) -> bool:
    if not isinstance(value, list):
        return False

    for v in value:
        if isinstance(v, dict):
            return False
        elif isinstance(v, list) and not is_list_without_dict(v):
            return False

    return True


def traverse_optional_args(
    obj: dict
) -> list[tuple[list[callable[[typing.Optional[list, dict]]]],
                callable[[typing.Any]]]]:
    ret = []
    for key, value in obj.items():
        if is_list_without_dict(value):
            ret.append(([operator.itemgetter(key)], itemsetter(obj, key)))

        elif isinstance(value, dict):
            for getters, setter in traverse_optional_args(value):
                ret.append(([operator.itemgetter(key)] + getters, setter))

        elif isinstance(value, list):
            for i, o in enumerate(value):
                for getters, setter in traverse_optional_args(o):
                    ret.append(([operator.itemgetter(i)] + getters, setter))

        else:
            raise ValueError('invalid case request')

    return ret


def combine_requests(req: dict) -> typing.Generator[dict, None, None]:
    optional_getsetter = traverse_optional_args(req)

    def combine_req_for_one(idx: int) -> typing.Generator[dict, None, None]:
        if idx == len(optional_getsetter):
            yield req
            return

        optional_args = req
        getters, setter = optional_getsetter[idx]
        for getter in getters:
            optional_args = getter(optional_args)

        for arg in optional_args:
            setter(arg)
            yield from combine_req_for_one(idx + 1)

        setter(optional_args)

    yield from combine_req_for_one(0)


def main():
    with open(sys.argv[1]) as f:
        rpcs = yaml.load(f)
    for rpc in rpcs:
        for req in combine_requests(rpc['requests']):
            print(rpc['method'])
            print(json.dumps(req))


if __name__ == '__main__':
    main()
