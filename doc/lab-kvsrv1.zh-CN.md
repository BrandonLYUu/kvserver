# 6.5840 实验 2：Key/Value 服务

## 简介

在本实验中，你将为单机实现一个键值（key/value）服务器，要求：尽管存在网络故障，每个 `Put` 操作仍至多执行一次（at-most-once），且各操作是线性一致（linearizable）的。你将用这个 KV 服务器实现一把锁。后面的实验会把这类服务器复制多份，以应对服务器崩溃。

## KV 服务器

每个客户端通过 *Clerk*（一组发送 RPC 给服务器的库函数）与键值服务器交互。客户端可向服务器发两种 RPC：`Put(key, value, version)` 与 `Get(key)`。服务器在内存 map 中为每个 key 记录一个（value，version）二元组。key 与 value 都是字符串，版本号（version number）记录该 key 被写过多少次。

`Put(key, value, version)` 仅当 Put 的版本号与服务器端该 key 的版本号一致时，才安装或替换 map 中该 key 的值；若一致，服务器还会把该 key 的版本号加一。若版本号不一致，服务器应返回 `rpc.ErrVersion`。客户端可用版本号 0 的 Put 创建新 key（服务器存下的版本号将为 1）。若 Put 的版本号大于 0 但 key 不存在，服务器应返回 `rpc.ErrNoKey`。

`Get(key)` 取回 key 的当前值与其版本号。若服务器上不存在该 key，应返回 `rpc.ErrNoKey`。

为每个 key 维护版本号有两个用处：用 Put 实现锁，以及在网络不可靠、客户端重传时保证 Put 的至多一次语义。

完成本实验并通过全部测试后，从调用 `Clerk.Get` / `Clerk.Put` 的客户端视角看，你将得到一个*线性一致*的键值服务。也就是说：若客户端操作互不并发，每个 `Clerk.Get` / `Clerk.Put` 都能看到此前操作序列隐含的状态修改；若操作并发，返回值与最终状态应等同于这些操作按某种顺序一次执行一个的效果。若操作在时间上重叠即为并发：例如客户端 X 调用 `Clerk.Put()` 的过程中，客户端 Y 也调用 `Clerk.Put()`，然后 X 的调用才返回。一个操作必须能看到在它开始之前已完成的所有操作的效果。更多背景见关于[线性一致性](https://pdos.csail.mit.edu/6.5840/papers/linearizability-faq.txt)的 FAQ。

线性一致性对应用很方便，因为它等同于单个服务器一次处理一个请求时的行为。例如，若某客户端收到服务器对更新请求的成功响应，则之后其他客户端发起的读必能看到这次更新的效果。对单服务器而言，提供线性一致性相对容易。

## 入门

骨架代码与测试在 `src/kvsrv1` 中。`kvsrv1/client.go` 实现了客户端用于管理与服务器 RPC 交互的 Clerk，提供 `Put` 与 `Get` 方法；`kvsrv1/server.go` 是服务器代码，含实现 RPC 服务端的 `Put` / `Get` 处理函数。你需要修改 `client.go` 与 `server.go`。RPC 请求、回复与错误值定义在 `kvsrv1/rpc` 包的 `kvsrv1/rpc/rpc.go` 中，看一下即可，无需修改。

按如下命令起步，别忘了 `git pull` 获取最新代码。

```
$ cd ~/6.5840
$ git pull
...
$ cd src
$ make kvsrv1
go build -race -o main/kvsrv1d main/kvsrv1d.go
cd kvsrv1 && go test -v -race  
=== RUN   TestReliablePut
One client and reliable Put (reliable network)...
    kvsrv_test.go:25: Put err ErrNoKey
--- FAIL: TestReliablePut (0.31s)
...
$
```

## 可靠网络下的键值服务器（难度：简单）

第一个任务是实现在消息不丢失时能工作的版本。你需要在 `client.go` 的 Clerk `Put`/`Get` 方法中补上发送 RPC 的代码，并在 `server.go` 中实现 `Put` / `Get` RPC 处理函数。

通过测试套件中的 Reliable 测试即算完成本任务：

```
$ cd src
$ make RUN="-run Reliable" kvsrv1
go build -race -o main/kvsrv1d main/kvsrv1d.go
cd kvsrv1 && go test -v -race -run Reliable 
=== RUN   TestReliablePut
One client and reliable Put (reliable network)...
  ... Passed --  time  0.0s #peers 1 #RPCs     5 #Ops    5
--- PASS: TestReliablePut (0.12s)
=== RUN   TestPutConcurrentReliable
Test: many clients racing to put values to the same key (reliable network)...
  ... Passed --  time  6.3s #peers 1 #RPCs 11025 #Ops 22050
--- PASS: TestPutConcurrentReliable (6.36s)
=== RUN   TestMemPutManyClientsReliable
Test: memory use many put clients (reliable network)...
  ... Passed --  time 29.0s #peers 1 #RPCs 50000 #Ops 50000
--- PASS: TestMemPutManyClientsReliable (52.91s)
PASS
ok      6.5840/kvsrv1   60.732s
$
```

每个 Passed 后的数字依次为：实际耗时（秒）、常量 1、发送的 RPC 数量（含客户端 RPC）、执行的键值操作数（Clerk `Get`/`Put` 调用数）。

## 用键值 Clerk 实现锁（难度：中等）

在很多分布式应用中，跑在不同机器上的客户端用键值服务器协调动作。例如 ZooKeeper 与 Etcd 允许客户端用分布式锁协调，类似于 Go 程序中线程用锁（如 `sync.Mutex`）协调。Zookeeper 与 Etcd 正是用条件 Put 实现这类锁的。

你的任务是实现锁（locks），把每把锁所需的状态存在键值服务器中。锁可以有多把，每把有独立名字，作为 `MakeLock` 的参数。锁支持两个方法：`Acquire` 与 `Release`。语义要求：同一时刻只有一台客户端能成功持有给定的锁；其他客户端必须等第一台客户端 `Release` 后才能拿到。

骨架代码与测试在 `src/kvsrv1/lock/` 中，你需要修改 `src/kvsrv1/lock/lock.go`。`Acquire` 与 `Release` 应通过调用 `lk.ck.Put()` / `lk.ck.Get()` 把锁状态存在键值服务器里。

若某客户端持有锁时崩溃，锁将永远无法释放。在比本实验更精细的设计中，客户端会给锁附上[租约（lease）](https://en.wikipedia.org/wiki/Lease_(computer_science)#:~:text=Leases%20are%20commonly%20used%20in,to%20rely%20on%20the%20resource.)，租约到期后锁服务器代客户端释放。本实验中客户端不会崩溃，可忽略该问题。

实现 `Acquire` 与 `Release`，通过以下测试即算完成：

```
$ cd src
$ make RUN="-run Reliable" lock1
go build -race -o main/kvsrv1d main/kvsrv1d.go
cd kvsrv1/lock; go test -v -race -run Reliable
=== RUN   TestReliableBasic
Test: a single Acquire and Release (reliable network)...
  ... Passed --  time  0.0s #peers 1 #RPCs     4 #Ops    4
--- PASS: TestReliableBasic (0.13s)
=== RUN   TestReliableNested
Test: one client, two locks (reliable network)...
  ... Passed --  time  0.1s #peers 1 #RPCs    17 #Ops   17
--- PASS: TestReliableNested (0.17s)
=== RUN   TestOneClientReliable
Test: 1 lock clients (reliable network)...
  ... Passed --  time  2.0s #peers 1 #RPCs   477 #Ops  477
--- PASS: TestOneClientReliable (2.14s)
=== RUN   TestManyClientsReliable
Test: 10 lock clients (reliable network)...
  ... Passed --  time  2.2s #peers 1 #RPCs  5704 #Ops 5704
--- PASS: TestManyClientsReliable (2.36s)
PASS
ok      6.5840/kvsrv1/lock      5.817s
$
```

若还没实现锁，前两个测试也会通过。

本练习代码量不大，但比上一个更需要独立思考。

- 每个锁客户端需要一个唯一标识，调 `kvtest.RandValue(8)` 生成随机串即可。

## 消息丢失下的键值服务器（难度：中等）

本练习的主要挑战是：网络可能重排、延迟或丢弃 RPC 请求与回复。为从丢弃中恢复，Clerk 必须不断重试每次 RPC，直到收到服务器回复。

若丢的是 RPC 请求报文，客户端重发即可解决问题：服务器只会收到并执行重发的那一份。

但若丢的是 RPC 回复报文就麻烦了。客户端只知道自己没收到回复，分不清丢的是请求还是回复。若丢的是回复而客户端重发请求，服务器就会收到同一请求的两份拷贝。对 `Get` 无妨，因为 `Get` 不改服务器状态。重发带相同版本号的 `Put` RPC 也是安全的，因为服务器按版本号条件执行 Put：若服务器已收到并执行过 Put，重发的拷贝只会得到 `rpc.ErrVersion`，而不会被执行第二次。

棘手的是：服务器对 Clerk 重发的 RPC 回了 `rpc.ErrVersion`。此时 Clerk 无法判断自己的 Put 是否已执行：可能是第一次 RPC 已被执行，只是成功的响应在网络中丢失，服务器仅对重发的 RPC 回了 `rpc.ErrVersion`；也可能是另一台 Clerk 先更新了该 key，Clerk 的两次 RPC 都没被执行，服务器对两次都回 `rpc.ErrVersion`。因此，若 Clerk 对重发过的 Put RPC 收到 `rpc.ErrVersion`，`Clerk.Put` 必须向应用返回 `rpc.ErrMaybe`（而非 `rpc.ErrVersion`），表示请求可能已执行，交由应用处理。若服务器对初次（未重发的）Put RPC 回 `rpc.ErrVersion`，则 Clerk 应向应用返回 `rpc.ErrVersion`，因为该 RPC 肯定没被执行。

若 Put 是精确一次（exactly-once，即没有 `rpc.ErrMaybe` 错误）对应用开发者更方便，但若不在服务器为每台 Clerk 维护状态就很难保证。本实验最后一个练习将用你的 Clerk 实现锁，体验在至多一次的 `Clerk.Put` 上编程。

现在修改 `kvsrv1/client.go`，使其在 RPC 请求/回复丢失时仍能推进。客户端 `ck.clnt.Call()` 返回 `true` 表示收到服务器 RPC 回复，`false` 表示没收到（更准确地说，`Call()` 等待回复一段时间，超时未到即返回 `false`）。你的 Clerk 应不断重发 RPC 直到收到回复。注意上文关于 `rpc.ErrMaybe` 的讨论。解答无需改动服务器。

给 Clerk 加上收不到回复即重试的代码。通过 `kvsrv1` 全部测试即算完成：

```
$ make kvsrv1
go build -race -o main/kvsrv1d main/kvsrv1d.go
cd kvsrv1 && go test -v -race  
=== RUN   TestReliablePut
One client and reliable Put (reliable network)...
  ... Passed --  time  0.0s #peers 1 #RPCs     5 #Ops    5
--- PASS: TestReliablePut (0.12s)
=== RUN   TestPutConcurrentReliable
Test: many clients racing to put values to the same key (reliable network)...
  ... Passed --  time  6.4s #peers 1 #RPCs 11021 #Ops 22042
--- PASS: TestPutConcurrentReliable (6.52s)
=== RUN   TestMemPutManyClientsReliable
Test: memory use many put clients (reliable network)...
  ... Passed --  time 28.8s #peers 1 #RPCs 50000 #Ops 50000
--- PASS: TestMemPutManyClientsReliable (52.44s)
=== RUN   TestUnreliableNet
One client (unreliable network)...
  ... Passed --  time  4.0s #peers 1 #RPCs   268 #Ops  422
--- PASS: TestUnreliableNet (4.13s)
PASS
ok      6.5840/kvsrv1   64.442s
$
```

- 客户端重试前应稍等一会，可用 Go 的 time 包调 `time.Sleep(100 * time.Millisecond)`。

## 不可靠网络下用键值 Clerk 实现锁（难度：简单）

修改你的锁实现，使其在网络不可靠时仍能配合改好的键值客户端正确工作。通过全部 `lock1` 测试即算完成：

```
$ make lock1
go build -race -o main/kvsrv1d main/kvsrv1d.go
cd kvsrv1/lock; go test -v -race 
=== RUN   TestReliableBasic
Test: a single Acquire and Release (reliable network)...
  ... Passed --  time  0.0s #peers 1 #RPCs     4 #Ops    4
--- PASS: TestReliableBasic (0.13s)
=== RUN   TestReliableNested
Test: one client, two locks (reliable network)...
  ... Passed --  time  0.1s #peers 1 #RPCs    17 #Ops   17
--- PASS: TestReliableNested (0.17s)
=== RUN   TestOneClientReliable
Test: 1 lock clients (reliable network)...
  ... Passed --  time  2.0s #peers 1 #RPCs   471 #Ops  471
--- PASS: TestOneClientReliable (2.13s)
=== RUN   TestManyClientsReliable
Test: 10 lock clients (reliable network)...
  ... Passed --  time  2.2s #peers 1 #RPCs  5658 #Ops 5658
--- PASS: TestManyClientsReliable (2.35s)
=== RUN   TestOneClientUnreliable
Test: 1 lock clients (unreliable network)...
  ... Passed --  time  2.1s #peers 1 #RPCs    66 #Ops   57
--- PASS: TestOneClientUnreliable (2.18s)
=== RUN   TestManyClientsUnreliable
Test: 10 lock clients (unreliable network)...
  ... Passed --  time  4.1s #peers 1 #RPCs   778 #Ops  617
--- PASS: TestManyClientsUnreliable (4.23s)
PASS
ok      6.5840/kvsrv1/lock      12.227s
$
```
