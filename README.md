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

## 必要な環境

- Go 1.16+
- Linux (epoll, forkのため)
- GCC (thread, hybridモードはcgoを使用)

## 参考文献

- [The C10K problem](http://www.kegel.com/c10k.html)
- [nginx architecture](https://www.nginx.com/blog/inside-nginx-how-we-designed-for-performance-scale/)
- [Go netpoller](https://morsmachine.dk/netpoller)
