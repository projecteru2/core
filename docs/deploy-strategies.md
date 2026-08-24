# Deploy strategies

A deploy request says *how many* workloads it wants, not *where*. The strategy turns the candidate
nodes into a plan — a `node -> count` map — and it is the only place that decision is made.

## The inputs

Before the strategy runs, core has collected for every candidate node:

| Field | Meaning |
| --- | --- |
| `Capacity` | How many more workloads of this shape fit, per the resource plugins (the minimum across them) |
| `Count` | How many of this app+entrypoint are already there — deployed **plus** in-flight |
| `Usage` | Current resource usage, weighted-averaged across plugins |
| `Rate` | What one more workload of this shape would consume, as a proportion of the node's total |

Plus `count` (how many to deploy), `nodes_limit` (`limit` below) and `total` (the sum of every
node's capacity).

`Count` including in-flight deploys is what makes concurrent deploys of the same app behave: core
writes a processing counter before creating anything, so a second deploy sees the first one's
intent.

## The strategies

| Name | Enum | Reads `limit` as | Behaviour |
| --- | --- | --- | --- |
| `AUTO` | `AUTO` (0) | max per node | Balance the cluster |
| `FILL` | `FILL` (1) | how many nodes | Top every node up to `count` |
| `EACH` | `EACH` (2) | how many nodes | Add `count` to every node |
| `GLOBAL` | `GLOBAL` (3) | ignored | Balance by resource usage |
| `DRAINED` | `DRAINED` (4) | ignored | Pack the tightest nodes first |
| `DUMMY` | `DUMMY` (99) | — | Not a plan; `CalculateCapacity` only |

### AUTO

The default. A min-heap over `(Count, -Capacity)`: repeatedly place one workload on the node with
the fewest instances, breaking ties toward the node with the most room, until `count` are placed.
The result is the flattest distribution reachable from wherever the cluster already is.

With `nodes_limit > 0` it is a per-node ceiling: a node already holding that many instances is not
a candidate. Nodes with zero capacity are excluded up front. Fails with `ErrInsufficientResource`
if total capacity is below `count`, or if the ceiling makes the request unsatisfiable.

Use it unless you have a reason not to.

### FILL

"Make sure each of `limit` nodes has `count` instances." Nodes are sorted by `(Count, Capacity)`
descending, and each one is topped up by `count - Count` (never negative) until `limit` nodes have
been covered. `limit = 0` means all nodes.

This is a convergence target, not an increment: re-running it is a no-op once satisfied, and it
returns `ErrAlreadyFilled` when it had nothing left to add. Use it for "keep N per node" services.

### EACH

"Add `count` more to each of `limit` nodes." Nodes are sorted by capacity descending, and the
top `limit` nodes each get exactly `count` — so the round deploys `count * limit` workloads.
`limit = 0` means all nodes. Every chosen node must have capacity for the full `count`, otherwise
the whole plan fails.

Use it for scaling a per-node daemon out by a fixed step.

### GLOBAL

A min-heap over `Usage + Rate`: each workload goes to the node that would be least loaded
*after* taking it, and that node's projected usage is updated before the next pick. Unlike `AUTO`,
which counts instances, this one balances resource pressure — the right choice when workloads of
the same app have very different footprints, or when the cluster's nodes are not uniform.

`nodes_limit` is ignored.

### DRAINED

The opposite of `GLOBAL`: nodes are sorted by capacity ascending (ties broken by higher usage) and
filled to their capacity one at a time. Consolidates workloads onto the fewest, tightest nodes and
leaves the roomy ones empty — useful before draining hardware, or to keep large contiguous
allocations possible elsewhere.

`nodes_limit` is ignored.

### DUMMY

Only meaningful for `CalculateCapacity`. It skips strategy selection entirely and returns each
node's raw capacity plus the total, which is what you want when asking "how much room is there?"
rather than "where would these go?".

## Filtering candidates

The strategy only sees nodes that survived `node_filter` on the request:

- `includes` — if non-empty, exactly these nodes, fetched by name. This short-circuits everything
  below: `excludes`, `labels` and the liveness check are not applied.
- otherwise, the nodes of `podname` (or every pod, if empty), then:
  - `labels` — the node must carry all of them
  - `all` — when false, nodes that are down are dropped
  - `excludes` — these names are removed from what is left

A node is *down* when it is marked `bypass` (via `SetNode`) or when its liveness key is absent,
which is what happens when its agent stops reporting. Nodes registered with `test: true` are
always treated as alive.
