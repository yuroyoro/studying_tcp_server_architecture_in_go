# TCP Server Architecture in Go

TCP Serverのアーキテクチャの歴史的な変化を学習するためのGoサンプルプログラム集です。

シンプルなEchoサーバーを題材に、各アーキテクチャの特徴と実装を比較できます。

## 実装されているアーキテクチャ

| モード | ファイル | 説明 |
|--------|----------|------|
| `simple` | simple.go | シングルプロセス・シングルスレッド |
| `fork` | fork.go | 接続ごとにfork |
| `prefork` | prefork.go | 事前にWorkerプロセスをfork |
| `thread` | thread.go | POSIXスレッド (pthread) |
| `asyncio` | asyncio.go | 非同期I/O (epoll) |
| `microthread` | microthread.go | Goのgoroutine |
| `hybrid` | hybrid.go | Multi-Reactor (epoll + pthread) |

## 使い方

```bash
# ビルド
go build -o server .

# 実行 (モード名を引数に指定)
./server <mode> [port]

# 例
./server simple 8080
./server fork 8080
./server prefork 8080
./server thread 8080
./server asyncio 8080
./server microthread 8080
./server hybrid 8080
```

## 動作確認

```bash
# 別ターミナルでクライアント接続
nc localhost 8080

# 文字を入力するとエコーバックされる
hello
hello
```

## 各アーキテクチャの解説

### 1. Simple (シングルプロセス)

最も基本的な実装。1つのプロセスが順番に接続を処理します。

```
Client A ──┐
           │  ┌─────────┐
           ├──│ Server  │  同時に1接続のみ処理可能
           │  └─────────┘
Client B ──┘  (waiting...)
```

**特徴:**
- 実装が最もシンプル
- 同時接続数は1のみ
- 1つのクライアントが接続中は他のクライアントは待機

### 2. Fork (接続ごとにfork)

接続を受け付けるたびに子プロセスをforkして処理を委譲します。

```
                    ┌─────────────┐
Client A ──────────│ Child Proc  │
                    └─────────────┘
  ┌─────────┐
  │ Parent  │ ─── accept() ───┐
  └─────────┘                 │
                    ┌─────────────┐
Client B ──────────│ Child Proc  │
                    └─────────────┘
```

**特徴:**
- 並行処理が可能
- プロセス間のメモリ空間が分離（安全）
- forkのオーバーヘッドが大きい
- プロセス数が増えるとリソース消費が増大

### 3. Prefork (事前fork)

起動時にWorkerプロセスを事前にforkしておき、各Workerが`accept()`を呼び出して接続を奪い合います。

```
                ┌──────────────┐
                │   Worker 0   │──┐
                └──────────────┘  │
  ┌─────────┐   ┌──────────────┐  │
  │ Parent  │───│   Worker 1   │──┼── 共有listenFD
  └─────────┘   └──────────────┘  │    accept()を競合
                ┌──────────────┐  │
                │   Worker 2   │──┘
                └──────────────┘
```

**特徴:**
- forkのオーバーヘッドを起動時に限定
- Apache HTTP Serverなどで採用
- Thundering Herd問題が発生しうる

### 4. Thread (POSIXスレッド)

接続ごとにpthreadを生成して処理します（cgoを使用）。

```
                    ┌─────────────┐
Client A ──────────│  pthread    │
                    └─────────────┘
  ┌─────────┐
  │  Main   │ ─── accept()
  └─────────┘
                    ┌─────────────┐
Client B ──────────│  pthread    │
                    └─────────────┘
```

**特徴:**
- プロセスよりも軽量
- メモリ空間を共有（データ共有が容易）
- スレッド生成のオーバーヘッドはforkより小さい
- C10K問題の原因となる

### 5. AsyncIO (非同期I/O / epoll)

シングルスレッドでepollを使い、複数の接続をイベント駆動で処理します。

```
  ┌─────────────────────────────────────┐
  │              epoll                  │
  │  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐   │
  │  │FD 1 │ │FD 2 │ │FD 3 │ │ ... │   │
  │  └─────┘ └─────┘ └─────┘ └─────┘   │
  └─────────────────────────────────────┘
                    │
                    ▼
              Event Loop
           (Single Thread)
```

**特徴:**
- 1スレッドで数万接続を処理可能
- コンテキストスイッチのオーバーヘッドがない
- コールバック地獄になりやすい
- Node.js, nginx, Redisなどで採用


### 6. Hybrid (Multi-Reactor)

Main ReactorがAcceptを担当し、複数のWorkerスレッド（Sub-Reactor）にラウンドロビンで振り分けます。各Workerは独自のepollを持ちます。

```
  ┌──────────────────────────────────────────────┐
  │                Main Reactor                  │
  │               (Go: accept)                   │
  └──────────────────────────────────────────────┘
           │            │            │
           ▼            ▼            ▼
    ┌──────────┐ ┌──────────┐ ┌──────────┐
    │ Worker 0 │ │ Worker 1 │ │ Worker 2 │
    │ (pthread)│ │ (pthread)│ │ (pthread)│
    │  epoll   │ │  epoll   │ │  epoll   │
    └──────────┘ └──────────┘ └──────────┘
```

**特徴:**
- 複数CPUコアを活用
- 各Workerが独立したイベントループを持つ
- Memcached, Nettyなどで採用
- 高スループットを実現

### 7. Microthread (goroutine)

Goのgoroutineを使った実装。内部的にはランタイムがepoll等を使用します。

```
                    ┌─────────────┐
Client A ──────────│  goroutine  │
                    └─────────────┘
  ┌─────────┐       ┌───────────────────┐
  │  Main   │       │   Go Runtime      │
  └─────────┘       │ (epoll + M:N調整)  │
                    └───────────────────┘
                    ┌─────────────┐
Client B ──────────│  goroutine  │
                    └─────────────┘
```

**特徴:**
- 軽量スレッド（数KB程度のスタック）
- ブロッキングI/Oのように書ける（シンプル）
- ランタイムが自動的にI/O多重化
- Go標準の手法

## アーキテクチャの歴史的変遷

```
Simple → Fork → Prefork → Thread → AsyncIO → Microthread/Hybrid
  │        │       │         │         │            │
1970s   1980s   1990s     2000s     2000s後半    2010s〜
```

1. **Simple**: 初期のサーバー実装
2. **Fork**: 並行処理の実現 (inetd, CGI)
3. **Prefork**: forkコスト削減 (Apache 1.x)
4. **Thread**: より軽量な並行処理 (Apache 2.x)
5. **AsyncIO**: C10K問題への対応 (nginx, Node.js)
6. **Microthread/Hybrid**: 開発効率と性能の両立 (Go, Erlang, Netty)

## HTTPプロトコルの進化とアーキテクチャへの影響

サーバーアーキテクチャの変遷とは別軸で、HTTPプロトコル自体も進化してきました。プロトコルの変化はサーバーアーキテクチャの設計に大きな影響を与えています。

### プロトコルの変遷

```
HTTP/1.0 → HTTP/1.1 → HTTP/2 → HTTP/3 (QUIC)
   │          │          │          │
 1996       1997       2015       2022
```

### HTTP/1.0 (1996)

```
Client                    Server
  │                         │
  │──── TCP Connect ───────▶│
  │──── GET /index.html ───▶│
  │◀─── Response ───────────│
  │◀─── TCP Close ──────────│
  │                         │
  │──── TCP Connect ───────▶│  (新しいリクエストごとに再接続)
  │──── GET /style.css ────▶│
  │◀─── Response ───────────│
  │◀─── TCP Close ──────────│
```

**特徴:**
- 1リクエスト = 1 TCP接続
- リクエスト完了後に接続を切断

**アーキテクチャへの影響:**
- Fork/Threadモデルでも問題なく動作
- 接続が短命なため、プロセス/スレッドはすぐ解放される
- TCPハンドシェイクのオーバーヘッドが大きい

### HTTP/1.1 (1997)

```
Client                    Server
  │                         │
  │──── TCP Connect ───────▶│
  │──── GET /index.html ───▶│
  │◀─── Response ───────────│
  │──── GET /style.css ────▶│  (同じ接続を再利用)
  │◀─── Response ───────────│
  │──── GET /app.js ───────▶│
  │◀─── Response ───────────│
  │         ...             │
  │◀─── TCP Close ──────────│  (タイムアウトまで維持)
```

**特徴:**
- **Keep-Alive**: 接続の永続化（デフォルト有効）
- **Pipelining**: 複数リクエストを連続送信（実用上は普及せず）
- **Host ヘッダ**: バーチャルホストの実現

**アーキテクチャへの影響:**
- 接続が長時間維持されるため、Thread/Forkモデルではリソースが枯渇
- **C10K問題を加速させた直接的な要因**
- Keep-Alive接続を効率的に扱うため、イベント駆動モデルへの移行が加速

```
HTTP/1.0時代:                    HTTP/1.1時代:
┌────────┐                      ┌────────┐
│Thread 1│→ 接続→処理→切断→解放   │Thread 1│→ 接続→処理→待機→処理→待機...
├────────┤                      ├────────┤
│Thread 2│→ 接続→処理→切断→解放   │Thread 2│→ 接続→処理→待機... (idle)
├────────┤                      ├────────┤
│Thread 3│→ 接続→処理→切断→解放   │Thread 3│→ 接続→処理→待機... (idle)
└────────┘                      └────────┘
  スレッドはすぐ解放               アイドル状態でもスレッドを占有
```

### HTTP/2 (2015)

```
Client                         Server
  │                              │
  │════ Single TCP Connection ═══│
  │                              │
  │──▶ Stream 1: GET /index.html │
  │──▶ Stream 2: GET /style.css  │  (同時に複数リクエスト)
  │──▶ Stream 3: GET /app.js     │
  │◀── Stream 2: Response ───────│  (順不同で返却)
  │◀── Stream 1: Response ───────│
  │◀── Stream 3: Response ───────│
  │                              │
```

**特徴:**
- **Multiplexing**: 単一TCP接続で複数ストリームを多重化
- **Binary Protocol**: テキストからバイナリへ（パース効率向上）
- **Header Compression (HPACK)**: ヘッダの圧縮
- **Server Push**: サーバからのプッシュ配信

**アーキテクチャへの影響:**
- 1クライアント = 1 TCP接続で済むため、接続数が激減
- **イベント駆動モデルとの親和性が極めて高い**
- ストリーム管理のステートマシンが必要（複雑化）
- バイナリフレームのパース処理が必要

```
HTTP/1.1:                        HTTP/2:
┌─────────────────────┐          ┌─────────────────────┐
│  Connection Pool    │          │  Single Connection  │
│ ┌───┐┌───┐┌───┐┌───┐│          │ ┌─────────────────┐ │
│ │C1 ││C2 ││C3 ││C4 ││          │ │   Multiplexer   │ │
│ └───┘└───┘└───┘└───┘│          │ │ S1  S2  S3  S4  │ │
└─────────────────────┘          │ └─────────────────┘ │
  複数接続が必要                  └─────────────────────┘
                                   1接続で全て処理
```

### HTTP/3 / QUIC (2022)

```
Client                         Server
  │                              │
  │════ QUIC (UDP) Connection ═══│
  │                              │
  │──▶ Stream 1: GET /index.html │
  │──▶ Stream 2: GET /style.css  │
  │◀── Stream 2: Response ───────│
  │    (Stream 1 パケロス発生)    │
  │◀── Stream 3: Response ───────│  ← Stream 1の再送を待たずに配信可能
  │◀── Stream 1: Response ───────│
  │                              │
```

**特徴:**
- **UDPベース**: TCPからUDPへ（カーネル依存からの脱却）
- **0-RTT接続確立**: 再接続時のハンドシェイク省略
- **独立したストリーム**: HOL (Head-of-Line) ブロッキングの解消
- **Connection Migration**: IPアドレスが変わっても接続維持
- **暗号化が必須**: TLS 1.3を統合

**アーキテクチャへの影響:**

1. **UDPソケットの管理**
   - TCPのような接続状態をカーネルが管理しない
   - アプリケーション層で接続状態を管理する必要がある
   - `accept()` の概念がなく、データグラム単位で処理

2. **暗号化処理の負荷**
   - 全通信が暗号化されるため、CPU負荷が増加
   - ハードウェアアクセラレーション（AES-NI等）の活用が重要

3. **イベント駆動モデルの必須化**
   - UDPは本質的にコネクションレス
   - 大量のストリームを効率的に処理するにはイベント駆動が必須

```
TCP (HTTP/1.1, HTTP/2):              QUIC (HTTP/3):
┌─────────────────────┐              ┌─────────────────────┐
│      Kernel         │              │    Application      │
│ ┌─────────────────┐ │              │ ┌─────────────────┐ │
│ │ TCP State Machine│ │              │ │QUIC State Machine│ │
│ │   (per conn)     │ │              │ │   (per conn)     │ │
│ └─────────────────┘ │              │ └─────────────────┘ │
└─────────────────────┘              └─────────────────────┘
  カーネルが接続管理                   アプリが接続管理
```

### プロトコル進化とアーキテクチャの相関

| プロトコル | 最適なアーキテクチャ | 理由 |
|-----------|---------------------|------|
| HTTP/1.0 | Fork / Thread | 短命な接続、シンプルな処理 |
| HTTP/1.1 | AsyncIO / Hybrid | Keep-Aliveで接続が長寿命化 |
| HTTP/2 | AsyncIO / Hybrid | Multiplexingでイベント駆動と相性◎ |
| HTTP/3 | AsyncIO / Hybrid | UDPでイベント駆動が必須 |

### 現代のHTTPサーバー実装例

| サーバー | アーキテクチャ | HTTP/2 | HTTP/3 |
|---------|---------------|--------|--------|
| nginx | Event-driven (epoll) | ✓ | ✓ |
| Caddy | goroutine (Go) | ✓ | ✓ |
| H2O | Multi-thread + Event | ✓ | ✓ |
| Node.js | Event-loop | ✓ | ✓ |

### まとめ

```
┌─────────────────────────────────────────────────────────────────┐
│                     アーキテクチャの進化                          │
│  Simple → Fork → Prefork → Thread → AsyncIO → Microthread      │
│                                        ▲                        │
│                                        │                        │
│                              ┌─────────┴─────────┐              │
│                              │  C10K問題への対応   │              │
│                              └─────────┬─────────┘              │
│                                        │                        │
│  ┌─────────────────────────────────────┴───────────────────┐    │
│  │                  プロトコルの進化                         │    │
│  │  HTTP/1.0 → HTTP/1.1 → HTTP/2 → HTTP/3                  │    │
│  │     │          │          │        │                    │    │
│  │   短命接続   Keep-Alive  多重化   UDPベース              │    │
│  │              (C10K加速)  (相性◎)  (必須化)               │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

プロトコルとアーキテクチャは相互に影響し合いながら進化してきました。HTTP/1.1のKeep-Aliveがイベント駆動モデルへの移行を促し、HTTP/2以降はイベント駆動モデルを前提とした設計になっています。

## Linux Kernelの進化とTCPサーバーアーキテクチャ

サーバーアーキテクチャの変遷は、HTTPプロトコルの進化だけでなく、Linux Kernelの進化にも強く影響されています。カーネルが提供するシステムコール、スケジューラ、ネットワークスタックの改善が、新しいアーキテクチャパターンを実現可能にしてきました。

### 全体像：Kernelの進化とアーキテクチャの対応

```
Linux Kernel           システムコール/機能           サーバーアーキテクチャ
─────────────────────────────────────────────────────────────────────
1.x    (1994)     fork, select                  Simple, Fork
  │                 │
2.0    (1996)     SMP対応                        Fork (マルチCPU活用)
  │                 │
2.2    (1999)     poll, sendfile                 Prefork
  │                 │
2.4    (2001)     clone改善, sendfile強化          Thread (per-connection)
  │                 │
2.6    (2003)     epoll, NPTL, O(1)スケジューラ    AsyncIO, Hybrid
  │                 │
2.6.23 (2007)     CFS                            ─┐
  │                 │                              │ goroutine等の
3.9    (2013)     SO_REUSEPORT                   ─┤ M:Nスレッドモデル
  │                 │                              │ が効率的に動作
4.5    (2016)     EBPF拡張                       ─┘
  │                 │
5.1    (2019)     io_uring                       次世代AsyncIO
  │                 │
6.x    (2022-)    io_uring拡張, zero-copy強化      高効率ハイブリッド
```

### 1. I/O多重化システムコールの進化

TCPサーバーアーキテクチャに最も直接的な影響を与えたのは、I/O多重化のためのシステムコールの進化です。

#### select (1983, BSD由来 → Linux 1.0)

```c
int select(int nfds, fd_set *readfds, fd_set *writefds,
           fd_set *exceptfds, struct timeval *timeout);
```

```
┌─────────────────────────────────────────────┐
│              select() の動作                 │
│                                             │
│  User Space          Kernel Space           │
│  ┌──────────┐        ┌──────────────┐       │
│  │ fd_set   │──copy──▶│ 全FDを線形   │       │
│  │ [0..1023]│        │ スキャン     │       │
│  │          │◀─copy──│ O(n)         │       │
│  └──────────┘        └──────────────┘       │
│                                             │
│  制約: FD_SETSIZE = 1024 (ハードコード)       │
│  毎回: fd_setをカーネルにコピー                │
│  毎回: 全FDを線形走査                         │
└─────────────────────────────────────────────┘
```

**制約:**
- 監視可能なFD数が最大1024（`FD_SETSIZE`）
- 呼び出しのたびにfd_setをユーザー空間⇔カーネル空間でコピー
- カーネル内で全FDを線形走査（O(n)）
- 数百接続を超えると性能が急激に劣化

**アーキテクチャへの影響:**
- 少数の接続を多重化するには十分だったが、大規模サーバーには不適
- 結果として、Fork/Preforkモデルが主流であり続けた

#### poll (1997, Linux 2.2)

```c
int poll(struct pollfd *fds, nfds_t nfds, int timeout);
```

**selectからの改善:**
- FD数の上限が撤廃（動的配列）
- よりクリーンなAPI（ビットマスクではなく構造体配列）

**残った問題:**
- 毎回全FDをカーネルにコピーする必要がある
- カーネル内での線形走査（O(n)）は変わらず
- 数千接続では依然としてオーバーヘッドが大きい

#### epoll (2002, Linux 2.6)

```c
int epoll_create(int size);
int epoll_ctl(int epfd, int op, int fd, struct epoll_event *event);
int epoll_wait(int epfd, struct epoll_event *events, int maxevents, int timeout);
```

```
┌──────────────────────────────────────────────────────────┐
│                   epoll の動作                            │
│                                                          │
│  User Space              Kernel Space                    │
│  ┌──────────┐            ┌──────────────────────┐        │
│  │          │            │  epoll instance      │        │
│  │ epoll_ctl│──1回登録──▶│  ┌────────────────┐  │        │
│  │ (ADD/MOD)│            │  │ Red-Black Tree │  │        │
│  │          │            │  │ (全監視FD)     │  │        │
│  │          │            │  └────────────────┘  │        │
│  │          │            │         │             │        │
│  │          │            │  デバイスからの       │        │
│  │          │            │  callback で通知      │        │
│  │          │            │         ▼             │        │
│  │          │            │  ┌────────────────┐  │        │
│  │epoll_wait│◀─ready分──│  │  Ready List    │  │        │
│  │          │   だけ返却  │  │  (準備済FDのみ) │  │        │
│  └──────────┘            │  └────────────────┘  │        │
│                          └──────────────────────┘        │
│                                                          │
│  ・FDの登録は1回だけ (毎回コピーしない)                      │
│  ・イベント発生時にcallbackでReady Listに追加               │
│  ・epoll_waitは準備済FDだけを返す → O(ready)               │
└──────────────────────────────────────────────────────────┘
```

**革命的な改善:**
- FDの登録は`epoll_ctl()`で1回だけ（毎回コピー不要）
- カーネル内部でRed-Black Treeにより管理
- イベント発生時にcallbackでReady Listに追加（線形走査不要）
- `epoll_wait()`はReady状態のFDだけを返す → **O(ready FD数)**
- Edge-Triggered (ET) / Level-Triggered (LT) モードの選択が可能

**アーキテクチャへの影響:**
- **C10K問題を解決する技術的基盤を提供**
- nginxやNode.jsの登場を可能にした
- AsyncIOアーキテクチャが現実的な選択肢に
- 本リポジトリの`asyncio.go`と`hybrid.go`はepollに依存

```
接続数とシステムコールの性能比較 (概念図):

性能
 ▲
 │  ████  select/poll: O(n)で劣化
 │  ████████
 │  ████████████
 │  ████████████████
 │  ░░░░░░░░░░░░░░░░░░  epoll: O(ready)でほぼ一定
 │  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
 └──────────────────────────────────────▶ 接続数
        1K     5K    10K    50K   100K
```

#### io_uring (2019, Linux 5.1)

```c
// Submission Queue (SQ) と Completion Queue (CQ) を共有メモリで管理
struct io_uring_sqe;  // Submission Queue Entry
struct io_uring_cqe;  // Completion Queue Entry
```

```
┌──────────────────────────────────────────────────────────────┐
│                    io_uring の動作                             │
│                                                              │
│  User Space                    Kernel Space                  │
│  ┌─────────────────────────────────────────────────┐         │
│  │          Shared Memory (mmap)                   │         │
│  │  ┌─────────────────┐  ┌─────────────────────┐  │         │
│  │  │ Submission Queue │  │ Completion Queue    │  │         │
│  │  │ ┌───┬───┬───┐   │  │ ┌───┬───┬───┐       │  │         │
│  │  │ │SQE│SQE│SQE│   │  │ │CQE│CQE│CQE│       │  │         │
│  │  │ └───┴───┴───┘   │  │ └───┴───┴───┘       │  │         │
│  │  │  User が書込     │  │  Kernel が書込       │  │         │
│  │  └─────────────────┘  └─────────────────────┘  │         │
│  └─────────────────────────────────────────────────┘         │
│                                                              │
│  ・システムコール呼び出し自体を最小化                             │
│  ・SQ/CQリングバッファでバッチ処理                               │
│  ・カーネルポーリングモード (SQPOLL) でsyscall 0回も可能          │
└──────────────────────────────────────────────────────────────┘
```

**epollからの改善:**
- ユーザー空間とカーネル空間で共有メモリのリングバッファを使用
- 複数のI/O操作をバッチでサブミット（システムコール回数を削減）
- `IORING_SETUP_SQPOLL`でカーネルスレッドがポーリング → **システムコール0回**での非同期I/O
- read/write/accept/connect等あらゆるI/Oを統一的に非同期化

**アーキテクチャへの影響:**
- epollベースのイベントループをさらに高効率化
- システムコールのオーバーヘッド自体を排除可能
- 今後のサーバーフレームワークの基盤技術

### 2. プロセス・スレッド管理の進化

#### fork() の改善：Copy-on-Write (Linux 1.0〜)

```
fork() の進化:

初期のfork:                         CoW (Copy-on-Write):
┌──────────┐   fork   ┌──────────┐  ┌──────────┐  fork  ┌──────────┐
│ Parent   │ ───────▶ │  Child   │  │ Parent   │ ────▶ │  Child   │
│ [データ] │          │ [コピー]  │  │ [データ] │       │ [共有]   │
└──────────┘          └──────────┘  └──────────┘       └──────────┘
                                           │                  │
全メモリを即座にコピー                        └──────┬───────────┘
→ 非常に遅い                                       │
                                            ページテーブルのみコピー
                                            書込時に初めて実コピー
                                            → fork自体は高速
```

**アーキテクチャへの影響:**
- CoWにより`fork()`のコストが劇的に削減
- Fork-per-connectionモデルが実用的な選択肢になった
- Apache 1.xのPreforkモデルが高性能を発揮できた理由の一つ

#### clone() とLinuxThreads → NPTL (Linux 2.6)

```
プロセスとスレッドの統一 (Linux):

┌────────────────────────────────────────────────────────┐
│  clone() システムコール                                  │
│                                                        │
│  clone(CLONE_VM | CLONE_FS | CLONE_FILES | ...)        │
│        ↓            ↓           ↓                      │
│    メモリ共有     FS共有      FD共有   → スレッド         │
│                                                        │
│  clone(0)                                              │
│    → 何も共有しない → fork相当 → プロセス                 │
│                                                        │
│  Linuxではスレッドもプロセスも内部的にはtask_struct        │
└────────────────────────────────────────────────────────┘
```

**LinuxThreads (Linux 2.0〜2.4) の問題:**
- スレッドごとに異なるPIDが割り振られる
- シグナル処理が壊れていた（POSIX非準拠）
- マネージャスレッドがボトルネック
- スレッド生成/破棄が遅い

**NPTL (Native POSIX Thread Library, Linux 2.6):**
- 同一プロセスのスレッドは同一TGIDを共有
- POSIX準拠のシグナル処理
- `futex`システムコールによる高効率な同期プリミティブ
- スレッド生成が約10倍高速化

```
LinuxThreads (2.4以前):              NPTL (2.6以降):
┌─────────────────────┐              ┌─────────────────────┐
│ Process (PID=100)   │              │ Process (TGID=100)  │
│                     │              │                     │
│ Thread1 (PID=101)   │              │ Thread1 (TID=100)   │
│ Thread2 (PID=102)   │              │ Thread2 (TID=101)   │
│ Thread3 (PID=103)   │              │ Thread3 (TID=102)   │
│                     │              │                     │
│ Manager (PID=100)   │              │ (マネージャ不要)     │
│ がスレッド管理       │              │                     │
│                     │              │                     │
│ ⚠ PIDがバラバラ     │              │ ✓ 同一TGID          │
│ ⚠ シグナル処理が不正 │              │ ✓ POSIX準拠         │
│ ⚠ 生成が遅い        │              │ ✓ futexで高速同期    │
└─────────────────────┘              └─────────────────────┘
```

**アーキテクチャへの影響:**
- NTPLにより Thread-per-connectionモデルが実用レベルに
- Apache 2.x (worker MPM) はNTPLの恩恵を直接受けた
- しかし、C10K規模ではスレッド数自体がボトルネックに → AsyncIOへ

#### futex (Fast Userspace Mutex, Linux 2.6)

```
従来のロック:                         futex:
┌──────────┐                         ┌──────────┐
│User Space│                         │User Space│
│          │  毎回                    │          │  競合なし:
│  lock()  │──syscall──▶ Kernel      │  lock()  │──atomic op──▶ 完了
│          │                         │          │  (syscall不要)
│          │                         │          │
│          │                         │          │  競合あり:
│          │                         │  lock()  │──syscall──▶ Kernel
│          │                         │          │  (必要な時だけ)
└──────────┘                         └──────────┘
```

**アーキテクチャへの影響:**
- マルチスレッドサーバーのロック性能が劇的に向上
- 競合がない場合はカーネルに入らずatomic操作で完了
- Hybridモデルのようなマルチスレッド×イベント駆動の実用性を向上

### 3. スケジューラの進化

Linuxのプロセススケジューラの進化は、サーバーが多数のプロセス/スレッドを扱う際の性能に直接影響しました。

#### O(n) スケジューラ (Linux 2.4以前)

```
Runqueue (全CPUで1つ):
┌───┬───┬───┬───┬───┬───┬───┬───┬───┬───┐
│ T │ T │ T │ T │ T │ T │ T │ T │ T │ T │  ← 全タスクをスキャン
└───┴───┴───┴───┴───┴───┴───┴───┴───┴───┘
         ↓ 毎回全走査して最高優先度を選択
         O(n) — タスク数に比例して遅くなる
```

**問題:**
- 実行キューが1つ（全CPUで共有）→ ロック競合
- 次に実行するタスクを選ぶのにO(n)
- 数百スレッドで顕著に劣化 → Fork/Threadモデルの限界を助長

#### O(1) スケジューラ (Linux 2.6, 2003)

```
Per-CPU Runqueue:
CPU 0                    CPU 1
┌─────────────────┐      ┌─────────────────┐
│ Active Array    │      │ Active Array    │
│ [pri 0] → T,T   │      │ [pri 0] → T     │
│ [pri 1] → T     │      │ [pri 1] → T,T,T │
│ [pri 2] →       │      │ [pri 2] → T     │
│    ...          │      │    ...          │
│ [pri 139]→      │      │ [pri 139]→      │
├─────────────────┤      ├─────────────────┤
│ Expired Array   │      │ Expired Array   │
│    ...          │      │    ...          │
└─────────────────┘      └─────────────────┘
      │                        │
      ▼                        ▼
bitmap で最高優先度を       bitmap で最高優先度を
O(1) で発見               O(1) で発見
```

**改善:**
- CPU毎にRunqueueを分離（ロック競合の排除）
- 優先度配列＋bitmapで次のタスクをO(1)で選択
- タスク数に関係なく一定時間でスケジューリング

**アーキテクチャへの影響:**
- 数千スレッドでもスケジューリングオーバーヘッドが一定
- Thread-per-connectionモデルの実用範囲を拡大（数千接続まで）
- しかし、スレッドのメモリ消費（スタック等）はスケジューラでは解決できない

#### CFS: Completely Fair Scheduler (Linux 2.6.23, 2007)

```
Red-Black Tree (実行時間でソート):

              ┌───────┐
              │ T(5ms)│  ← vruntime が最小のタスクを
              └───┬───┘     左端から O(log n) で取得
             ╱         ╲
       ┌───────┐    ┌───────┐
       │ T(3ms)│    │ T(8ms)│
       └───┬───┘    └───────┘
      ╱         ╲
┌───────┐    ┌───────┐
│ T(1ms)│    │ T(4ms)│
└───────┘    └───────┘
  ↑
  最小vruntime = 次に実行
```

**O(1)スケジューラからの改善:**
- vruntimeベースの公平なCPU時間配分
- Red-Black Treeによる O(log n) のタスク選択
- ヒューリスティクスの削減で予測可能な動作
- ワークロードの種類を問わず安定した性能

**アーキテクチャへの影響:**
- goroutineのようなM:Nスレッドモデルのランタイムがカーネルスレッドを効率的に利用可能に
- 公平なスケジューリングにより、I/O boundとCPU boundのタスクが混在するサーバーでも安定動作
- Go runtimeはP (Processor) × M (Machine=OSスレッド) をCFSに委ね、G (Goroutine) の管理をユーザー空間で行う

```
Go Runtime と CFS の協調:

┌────────────────────────────────────────┐
│              User Space                │
│  ┌──────────────────────────────────┐  │
│  │         Go Runtime               │  │
│  │  G G G G G G G G  (goroutines)   │  │
│  │  ↓ ↓ ↓ ↓                        │  │
│  │  P₀    P₁    P₂    P₃  (GOMAXPROCS) │
│  │  ↓     ↓     ↓     ↓            │  │
│  │  M₀    M₁    M₂    M₃  (OS threads) │
│  └──────────────────────────────────┘  │
├────────────────────────────────────────┤
│              Kernel Space              │
│  ┌──────────────────────────────────┐  │
│  │        CFS Scheduler             │  │
│  │  M₀, M₁, M₂, M₃ を公平にスケジュール │
│  └──────────────────────────────────┘  │
└────────────────────────────────────────┘
```

### 4. ネットワークスタックの進化

#### sendfile() (Linux 2.2) と splice() (Linux 2.6.17)

```
従来のファイル送信:                    sendfile():
┌──────────┐                         ┌──────────┐
│User Space│                         │User Space│
│          │  read()                  │          │
│  buf[]   │◀──────── Kernel         │sendfile()│──────▶ Kernel
│          │                         │(1 syscall)│
│          │  write()                │          │      ┌──────┐
│  buf[]   │────────▶ Kernel         │          │      │Disk  │
└──────────┘                         └──────────┘      │  ↓   │
                                                       │Socket│
4回のコンテキストスイッチ                                └──────┘
2回のデータコピー                                    カーネル内で直接転送
(User↔Kernel)                                      ユーザー空間のコピー不要
```

**アーキテクチャへの影響:**
- 静的ファイル配信の効率が劇的に向上
- nginxが高速な静的ファイル配信を実現できた技術的基盤
- zero-copy技術の先駆け

#### SO_REUSEPORT (Linux 3.9, 2013)

```
従来: 1つのプロセスがaccept()                SO_REUSEPORT:
┌──────────┐                               ┌──────────┐ ┌──────────┐ ┌──────────┐
│ Process  │ ← accept()                    │Worker 0  │ │Worker 1  │ │Worker 2  │
│  (1つ)   │    ボトルネック                 │listen:80 │ │listen:80 │ │listen:80 │
└──────────┘                               └──────────┘ └──────────┘ └──────────┘
     │                                          │            │            │
     ▼                                     カーネルが接続を分散（ハッシュベース）
 全接続をさばく                                   │            │            │
                                            ┌────┴────────────┴────────────┴────┐
                                            │         Kernel (port 80)          │
                                            │    accept()を各Workerに分散       │
                                            └──────────────────────────────────┘
```

**従来の問題:**
- Preforkモデルでは複数Workerが同じlistenソケットの`accept()`を競合
- **Thundering Herd問題**: 1つの接続に全Workerが起こされる
- acceptのロック競合がボトルネック

**SO_REUSEPORTの解決:**
- 同一ポートに複数のlistenソケットをバインド可能
- カーネルが接続元のハッシュに基づきソケットを選択
- Thundering Herd問題を根本的に解消
- ロック競合なし

**アーキテクチャへの影響:**
- Preforkモデルの性能問題を解消
- nginx 1.9.1以降で採用、性能が大幅に向上
- マルチプロセス/マルチスレッドのイベント駆動サーバー（Hybrid型）に特に有効

#### accept4() (Linux 2.6.28)

```c
// 従来: accept() + fcntl() で2回のsyscall
int fd = accept(listen_fd, &addr, &addrlen);
fcntl(fd, F_SETFL, O_NONBLOCK);
fcntl(fd, F_SETFD, FD_CLOEXEC);

// accept4(): 1回のsyscallでフラグ設定
int fd = accept4(listen_fd, &addr, &addrlen, SOCK_NONBLOCK | SOCK_CLOEXEC);
```

**アーキテクチャへの影響:**
- AsyncIOサーバーでは新規接続のたびにnon-blocking設定が必要
- accept4()によりシステムコール回数を削減（高頻度accept時に効果大）
- レースコンディション（accept〜fcntl間にforkした場合のFD漏洩）を防止

#### TCP_FASTOPEN (Linux 3.7, 2012)

```
通常のTCPハンドシェイク:           TCP Fast Open:
Client        Server              Client        Server
  │               │                │               │
  │──── SYN ─────▶│                │── SYN+Data ──▶│  ← 最初から
  │◀─── SYN-ACK ──│                │◀─ SYN-ACK+Resp│    データ送信
  │──── ACK ──────▶│                │── ACK ────────▶│
  │──── Data ─────▶│  (3 RTT)      │                │  (1 RTT)
  │◀─── Response ──│                │                │
```

**アーキテクチャへの影響:**
- 短命な接続（HTTP/1.0的なワークロード）の効率を改善
- 接続確立のレイテンシを1 RTT削減
- CDNやAPIサーバーで効果を発揮

### 5. Kernelの進化が各アーキテクチャに与えた影響のまとめ

| Kernel機能 | 登場 | 影響を受けたアーキテクチャ | 影響の内容 |
|-----------|------|------------------------|-----------|
| Copy-on-Write | 初期 | Fork, Prefork | fork()の高速化で実用的に |
| select | BSD由来 | Simple → 初期のAsyncIO | 最初のI/O多重化だが限界あり |
| poll | 2.2 | AsyncIO | FD上限を撤廃、しかしO(n)は未解決 |
| sendfile | 2.2 | 全般 | 静的配信のzero-copy化 |
| **epoll** | **2.6** | **AsyncIO, Hybrid** | **C10K解決の核心技術** |
| **NPTL** | **2.6** | **Thread, Hybrid** | **スレッドモデルを実用レベルに** |
| **O(1) Sched** | **2.6** | **Thread, Hybrid** | **大量スレッドでのスケジューリング高速化** |
| futex | 2.6 | Thread, Hybrid | ユーザー空間ロックの高速化 |
| CFS | 2.6.23 | Microthread (goroutine) | M:Nモデルの基盤、公平なスケジューリング |
| accept4 | 2.6.28 | AsyncIO, Hybrid | accept時のsyscall削減 |
| TCP_FASTOPEN | 3.7 | 全般 | 接続確立の高速化 |
| **SO_REUSEPORT** | **3.9** | **Prefork, Hybrid** | **Thundering Herd問題の解消** |
| **io_uring** | **5.1** | **次世代AsyncIO** | **syscall自体のオーバーヘッド排除** |

### 6. Kernelの進化がなければ何が起きなかったか

```
┌────────────────────────────────────────────────────────────────────┐
│                                                                    │
│  もしepollがなかったら:                                               │
│    → nginx, Node.js, Redis のようなイベント駆動サーバーは              │
│      Linux上で高性能を発揮できなかった                                 │
│    → FreeBSD の kqueue が先行し、Linux は劣位に                      │
│                                                                    │
│  もしNTPLがなかったら:                                                │
│    → Thread-per-connectionモデルは数百接続が限界                      │
│    → Java/Apache のスレッドモデルがLinuxで不利に                       │
│                                                                    │
│  もしCFSがなかったら:                                                 │
│    → Go runtimeのようなM:Nスケジューラが                              │
│      カーネルスレッドを効率的に利用できなかった                          │
│                                                                    │
│  もしSO_REUSEPORTがなかったら:                                        │
│    → マルチコア環境でのaccept()がボトルネックのまま                     │
│    → Prefork/Hybridモデルの性能向上が限定的に                         │
│                                                                    │
│  もしio_uringがなかったら:                                            │
│    → システムコールのオーバーヘッドが性能の壁として残り続ける             │
│    → 超高負荷環境でのさらなる最適化が困難に                             │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

Kernelの進化は単なる性能改善ではなく、**新しいアーキテクチャパターンを生み出す原動力**でした。epollなしにnginxは生まれず、NTPLなしにApache 2.xのworker MPMは機能せず、CFSなしにGoのgoroutineモデルは現在の効率を達成できませんでした。サーバーアーキテクチャの歴史は、Kernelの機能追加に対するアプリケーション層の適応の歴史でもあります。

## 必要な環境

- Go 1.16+
- Linux (epoll, forkのため)
- GCC (thread, hybridモードはcgoを使用)

## 参考文献

- [The C10K problem](http://www.kegel.com/c10k.html)
- [nginx architecture](https://www.nginx.com/blog/inside-nginx-how-we-designed-for-performance-scale/)
- [Go netpoller](https://morsmachine.dk/netpoller)
- [epoll(7) - Linux man page](https://man7.org/linux/man-pages/man7/epoll.7.html)
- [io_uring - Efficient IO with io_uring (kernel.dk)](https://kernel.dk/io_uring.pdf)
- [NPTL - Native POSIX Thread Library](https://man7.org/linux/man-pages/man7/pthreads.7.html)
- [SO_REUSEPORT (lwn.net)](https://lwn.net/Articles/542629/)
- [CFS Scheduler Design](https://www.kernel.org/doc/html/latest/scheduler/sched-design-CFS.html)
